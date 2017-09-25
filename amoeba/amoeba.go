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
    amoebaBuild = "com.amoeba.build"
    outDir = "./out"
    buildsDir = "./out/builds"
    logsDir   = "./out/logs"
)

// Amoeba ...
type Amoeba struct {
    docker *client.Client
    ctx context.Context
    url string
    id  string
    cmds []*exec.Cmd
}

// output ...
type Output struct {
    Name string
    Stdout io.Reader
    Stderr io.Reader
}

// NewAmoeba ...
func NewAmoeba(url string, id string) (*Amoeba, error) {
    docker, err := client.NewEnvClient()
    ctx := context.Background()

    return &Amoeba{docker: docker, ctx: ctx, url: url, id: id}, err
}

// Close ...
func (a *Amoeba) Close() {
    a.docker.Close()
}

// Start ...
func (a *Amoeba) Start() ([]Output) {
    var wg sync.WaitGroup

    buildPath := filepath.Join(buildsDir, a.id)

    utils.CheckDir(outDir)
    utils.CheckDir(buildsDir)
    utils.CheckDir(buildPath)
    utils.CheckDir(logsDir)
    utils.CheckDir(filepath.Join(logsDir, a.id))

    buildElements := strings.Split(a.id, "-")
    repoName := buildElements[0]
    commitID := buildElements[1]

    // Clone target repo
    log.Println("Cloning repository: " + repoName)
    targetPath := filepath.Join(buildPath, "target")
    repo.CloneRepo(a.url, targetPath, commitID)

    // Archive repo for docker build context
    buildContext := repo.ArchiveRepo(targetPath)
    defer buildContext.Close()

    // Build image
    log.Println("Building image: " + a.id)

    wg.Add(1)
    go func() {
        defer wg.Done()
        a.buildImage(buildContext)
    }()

    clients := repo.ParseConfig(targetPath)
    a.setupClients(clients, repoName)
    wg.Wait()

    return a.startClients(clients)
}

// Wait ...
func (a *Amoeba) Wait() ([]error) {
    var errs []error

    for _, cmd := range a.cmds {
        log.Println("Waiting for cmd")
        err := cmd.Wait()
        log.Println("Waiting done")
        if err != nil {
            errs = append(errs, err)
        }
    }

    buildPath := filepath.Join(buildsDir, a.id)
    dirs, err := ioutil.ReadDir(buildPath)
    utils.CheckError(err)

    for _, dir := range dirs {
        name := dir.Name()
        if strings.Contains(name, "client") {
            dockerComposeDown(filepath.Join(buildPath, name))
        }
    }

    // Remove build build directory
    log.Println("Removing build directory: " + a.id)
    err = os.RemoveAll(buildPath) // Remove all files
    utils.CheckError(err)

    // Remove built image
    log.Println("Removing image: " + a.id)
    a.removeImage() // Remove built image

    return errs
}

// Builds the image from the given buildContext and assigns the image the
// given name. User responsible for closing buildContext.
func (a *Amoeba) buildImage(buildContext io.Reader) {
    opts := types.ImageBuildOptions{
        Tags: []string{a.id + ":latest"},
        Remove: true,
        ForceRemove: true,
        Labels: map[string]string{amoebaBuild: a.id},
    }

    res, err := a.docker.ImageBuild(a.ctx, buildContext, opts)
    utils.CheckError(err)
    defer res.Body.Close()

    _, err = ioutil.ReadAll(res.Body) // Blocks until the image is built
    utils.CheckError(err)
}

// Remove the image by the given name from the docker daemon.
func (a *Amoeba) removeImage() {
    options := types.ImageListOptions{All: true}
    images, err := a.docker.ImageList(a.ctx, options)
    utils.CheckError(err)

    for _, image := range images {
        if image.Labels[amoebaBuild] == a.id {
            opts := types.ImageRemoveOptions{
                Force: true,
                PruneChildren: true,
            }

            _, err := a.docker.ImageRemove(a.ctx, image.ID, opts)
            utils.CheckError(err)
        }
    }
}

func (a *Amoeba) setupClients(clients []string, repoName string) {
    var wg sync.WaitGroup

    path := filepath.Join(buildsDir, a.id)

    for i, url := range clients {
        wg.Add(1)
        go func() {
            defer wg.Done()
            repoPath := filepath.Join(path, "client" + strconv.Itoa(i))
            log.Println("Cloning client repository: " + url)
            repo.CloneRepo(url, repoPath, "")
            log.Println("Generating docker-compose.override.yml")
            repo.GenOverride(repoPath, repoName, a.id)
        }()
    }

    wg.Wait()
}

func (a *Amoeba) startClients(clients []string) ([]Output) {
    var outputs []Output

    path := filepath.Join(buildsDir, a.id)

    for i, url := range clients {
        repoPath := filepath.Join(path, "client" + strconv.Itoa(i))
        log.Println("docker compose up on " + url)
        cmd, output := dockerComposeUp(a.id, repo.ParseName(url), repoPath)
        outputs = append(outputs, output)
        a.cmds = append(a.cmds, cmd)
    }

    return outputs
}

func dockerComposeUp(id, repo, dir string) (*exec.Cmd, Output) {
    return dockerComposeOut(id, repo, dir, "up", "--abort-on-container-exit")
    //return dockerComposeOut(id, repo, dir, "up", "-d")
}

func dockerComposeDown(dir string) error {
    return dockerCompose(dir, "down", "--remove-orphans")
}

func dockerComposeOut(id string, repo string, dir string, args ...string) (*exec.Cmd, Output) {
    output := Output{}
    output.Name = repo
    path := filepath.Join(logsDir, id, repo)
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

    outPr, outPw := io.Pipe()
    outTr := io.TeeReader(stdout, outPw)

    errPr, errPw := io.Pipe()
    errTr := io.TeeReader(stderr, errPw)

    output.Stdout = outTr
    output.Stderr = errTr

    err = cmd.Start()
    utils.CheckError(err)

    go io.Copy(stdoutFile, outPr)
    go io.Copy(stderrFile, errPr)

    return cmd, output
}

func dockerCompose(dir string, args ...string) error {
    cmd := exec.Command("docker-compose", args...)
    cmd.Dir = dir

    return cmd.Run()
}
