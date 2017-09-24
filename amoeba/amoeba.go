package amoeba

import (
    "os"
    "io"
    "log"
    "sync"
    "strconv"
    "os/exec"
    "io/ioutil"
    "strings"
    "path/filepath"
    "golang.org/x/net/context"
    "github.com/docker/docker/api/types"
    "github.com/docker/docker/client"
    "amoeba/repo"
    "amoeba/utils"
)

const (
    amoebaGroup = "com.amoeba.group"
    groupsDir = "./groups"
    outDir = "./out"
)

// Amoeba ...
type Amoeba struct {
    ctx context.Context
    docker *client.Client
}

// Stream ...
type Stream struct {
    Name string
    Stdout io.Reader
    Stderr io.Reader
}

// NewAmoeba ...
func NewAmoeba() (*Amoeba, error) {
    docker, err := client.NewEnvClient()
    ctx := context.Background()

    return &Amoeba{ctx, docker}, err
}

// Close ...
func (a *Amoeba) Close() {
    a.docker.Close()
}

// StartDeploy ...
func (a *Amoeba) StartDeploy(url string, group string) ([]*exec.Cmd, []Stream) {
    var wg sync.WaitGroup

    groupPath := filepath.Join(groupsDir, group)

    utils.CheckDir(groupsDir)
    utils.CheckDir(groupPath)
    utils.CheckDir(outDir)
    utils.CheckDir(filepath.Join(outDir, group))

    groupElements := strings.Split(group, "-")
    repoName := groupElements[0]
    commitID := groupElements[1]

    // Clone target repo
    log.Println("Cloning repository: " + repoName)
    targetPath := filepath.Join(groupPath, "target")
    repo.CloneRepo(url, targetPath, commitID)

    // Archive repo for docker build context
    buildContext := repo.ArchiveRepo(targetPath)
    defer buildContext.Close()

    // Build image
    log.Println("Building image: " + group)

    wg.Add(1)
    go func() {
        defer wg.Done()
        a.buildImage(buildContext, group)
    }()

    clients := repo.ParseConfig(targetPath)
    setupClients(clients, repoName, group)
    wg.Wait()

    return startClients(clients, group)
}

// WaitDeploy ...
func (a *Amoeba) WaitDeploy(cmds []*exec.Cmd) ([]error) {
    var errs []error
    for _, cmd := range cmds {
        log.Println("Waiting for command from: " + cmd.Dir)
        err := cmd.Wait()
        if err != nil {
            log.Println("Command had an error")
            errs = append(errs, err)

        } else {
            log.Println("Command had no error")
        }
    }

    return errs
}

// TearDownGroup ...
func (a *Amoeba) TearDownDeploy(group string) {
    groupPath := filepath.Join(groupsDir, group)
    dirs, err := ioutil.ReadDir(groupPath)
    utils.CheckError(err)

    for _, dir := range dirs {
        name := dir.Name()
        if strings.Contains(name, "client") {
            dockerComposeDown(filepath.Join(groupPath, name))
        }
    }

    // Remove group build directory
    log.Println("Removing group directory: " + group)
    err = os.RemoveAll(groupPath) // Remove all files
    utils.CheckError(err)

    // Remove built image
    log.Println("Removing image: " + group)
    a.removeImage(group) // Remove built image
}

// Builds the image from the given buildContext and assigns the image the
// given name. User responsible for closing buildContext.
func (a *Amoeba) buildImage(buildContext io.Reader, name string) {
    opts := types.ImageBuildOptions{
        Tags: []string{name + ":latest"},
        Remove: true,
        ForceRemove: true,
        Labels: map[string]string{amoebaGroup: name},
    }

    res, err := a.docker.ImageBuild(a.ctx, buildContext, opts)
    utils.CheckError(err)
    defer res.Body.Close()

    _, err = ioutil.ReadAll(res.Body) // Blocks until the image is built
    utils.CheckError(err)
}

// Remove the image by the given name from the docker daemon.
func (a *Amoeba) removeImage(name string) {
    options := types.ImageListOptions{All: true}
    images, err := a.docker.ImageList(a.ctx, options)
    utils.CheckError(err)

    for _, image := range images {
        if image.Labels[amoebaGroup] == name {
            opts := types.ImageRemoveOptions{
                Force: true,
                PruneChildren: true,
            }

            _, err := a.docker.ImageRemove(a.ctx, image.ID, opts)
            utils.CheckError(err)
        }
    }
}

func setupClients(clients []string, repoName string, group string) {
    var wg sync.WaitGroup

    path := filepath.Join(groupsDir, group)

    for i, url := range clients {
        wg.Add(1)
        go func() {
            defer wg.Done()
            repoPath := filepath.Join(path, "client" + strconv.Itoa(i))
            log.Println("Cloning client repository: " + url)
            repo.CloneRepo(url, repoPath, "")
            log.Println("Generating docker-compose.override.yml")
            repo.GenOverride(repoPath, repoName, group)
        }()
    }

    wg.Wait()
}

func startClients(clients []string, group string) ([]*exec.Cmd, []Stream) {
    var wg sync.WaitGroup
    var streams []Stream
    var cmds []*exec.Cmd

    path := filepath.Join(groupsDir, group)

    for i, url := range clients {
        wg.Add(1)
        go func() {
            defer wg.Done()
            repoPath := filepath.Join(path, "client" + strconv.Itoa(i))
            log.Println("docker compose up on " + url)
            cmd, stream := dockerComposeUp(group, parseName(url), repoPath)
            streams = append(streams, stream)
            cmds = append(cmds, cmd)
        }()
    }

    wg.Wait()
    return cmds, streams
}

func parseName(url string) string {
    temp := strings.Split(url, "/")
    return strings.Split(temp[len(temp) - 1], ".")[0]
}

func dockerComposeUp(group, repo, dir string) (*exec.Cmd, Stream) {
    return dockerComposeOut(group, repo, dir, "up", "--abort-on-container-exit")
    // return dockerComposeOut(group, repo, dir, "up", "-d")
}

func dockerComposeDown(dir string) error {
    return dockerCompose(dir, "down", "--remove-orphans")
}

// TODO pipe output to files and to websocket
func dockerComposeOut(group string, repo string, dir string, args ...string) (*exec.Cmd, Stream) {
    stream := Stream{}
    stream.Name = repo
    path := filepath.Join(outDir, group, repo)
    utils.CheckDir(path)

    stdoutFile, err := os.Create(filepath.Join(path, "stdout"))
    utils.CheckError(err)
    stderrFile, err := os.Create(filepath.Join(path, "stderr"))
    utils.CheckError(err)

    cmd := exec.Command("docker-compose", args...)
    cmd.Dir = dir

    stdout, err := cmd.StdoutPipe()
    utils.CheckError(err)
    stderr, err := cmd.StderrPipe()
    utils.CheckError(err)

    stream.Stdout = stdout
    stream.Stderr = stderr

    err = cmd.Start()
    go io.Copy(stdoutFile, stdout)
    go io.Copy(stderrFile, stderr)
    
    return cmd, stream
}

func dockerCompose(dir string, args ...string) error {
    cmd := exec.Command("docker-compose", args...)
    cmd.Dir = dir

    return cmd.Run()
}
