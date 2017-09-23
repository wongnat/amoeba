package amoeba

import (
    "os"
    "io"
    "log"
    "sync"
    //"bytes"
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
func (a *Amoeba) StartDeploy(url string, group string) []error {
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

func startClients(clients []string, group string) []error {
    var wg sync.WaitGroup
    var errs []error

    path := filepath.Join(groupsDir, group)

    for i, url := range clients {
        wg.Add(1)
        go func() {
            defer wg.Done()
            repoPath := filepath.Join(path, "client" + strconv.Itoa(i))
            log.Println("docker compose up on " + url)
            err := dockerComposeUp(group, parseName(url), repoPath)
            errs = append(errs, err)
        }()
    }

    wg.Wait()
    return errs
}

func parseName(url string) string {
    temp := strings.Split(url, "/")
    return strings.Split(temp[len(temp) - 1], ".")[0]
}

func dockerComposeUp(group, repo, dir string) error {
    return dockerComposeOut(group, repo, dir, "up", "--abort-on-container-exit")
    // return dockerComposeOut(group, repo, dir, "up", "-d")
}

func dockerComposeDown(dir string) error {
    return dockerCompose(dir, "down", "--remove-orphans")
}

func dockerComposeOut(group string, repo string, dir string, args ...string) error {
    path := filepath.Join(outDir, group, repo)
    utils.CheckDir(path)

    stdout, err := os.Create(filepath.Join(path, "stdout"))
    utils.CheckError(err)
    stderr, err := os.Create(filepath.Join(path, "stderr"))
    utils.CheckError(err)

    cmd := exec.Command("docker-compose", args...)
    cmd.Dir = dir
    cmd.Stdout = stdout
    cmd.Stderr = stderr

    return cmd.Run()
}

func dockerCompose(dir string, args ...string) error {
    cmd := exec.Command("docker-compose", args...)
    cmd.Dir = dir

    return cmd.Run()
}
