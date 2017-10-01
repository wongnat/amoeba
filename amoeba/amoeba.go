package amoeba

import (
    "os"
    "io"
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

// TODO: implement error handling instead of just panicking

const amoebaBuild = "com.amoeba.build" // Label key for docker images built

// Amoeba ...
type Amoeba struct {
    cli *client.Client
    ctx context.Context
    url string
    bid string
    dir string
    cmds []*exec.Cmd
}

// Output ...
type Output struct {
    Name string
    Stdout io.Reader
    Stderr io.Reader
}

// NewAmoeba ...
func NewAmoeba(url, sha, dir string) (*Amoeba, error) {
    docker, err := client.NewEnvClient()

    bid := repo.ParseName(url) + "-" + sha
    ctx := context.Background()

    a := Amoeba{
        cli: docker,
        ctx: ctx,
        url: url,
        bid: bid,
        dir: dir,
    }

    return &a, err
}

// Close ...
func (a *Amoeba) Close() {
    a.cli.Close()
}

// Start ...
func (a *Amoeba) Start() ([]Output) {
    var wg sync.WaitGroup

    buildPath := filepath.Join(a.dir, a.bid)

    utils.CheckDir(a.dir)
    utils.CheckDir(buildPath)

    sha  := strings.Split(a.bid, "-")[1]
    dest := filepath.Join(buildPath, "target")

    repo.Clone(a.url, sha, dest)

    buildContext := repo.Archive(dest)
    defer buildContext.Close()

    wg.Add(1)
    go func() {
        defer wg.Done()
        a.buildImage(buildContext)
    }()

    clients := repo.ParseConfig(dest)
    a.setupClients(clients)
    wg.Wait()

    return a.startClients(clients)
}

// Wait ...
func (a *Amoeba) Wait() ([]error) {
    var errs []error

    for _, cmd := range a.cmds {
        err := cmd.Wait()
        if err != nil {
            errs = append(errs, err)
        }
    }

    buildPath := filepath.Join(a.dir, a.bid)
    dirs, err := ioutil.ReadDir(buildPath)
    utils.CheckError(err)

    for _, dir := range dirs {
        name := dir.Name()
        if strings.Contains(name, "client") {
            dockerComposeDown(filepath.Join(buildPath, name))
        }
    }

    err = os.RemoveAll(buildPath)
    utils.CheckError(err)
    a.removeImage()

    return errs
}

// Builds the image from the given buildContext.
func (a *Amoeba) buildImage(buildContext io.Reader) {
    opts := types.ImageBuildOptions{
        Tags: []string{a.bid + ":latest"},
        Remove: true,
        ForceRemove: true,
        Labels: map[string]string{amoebaBuild: a.bid},
    }

    res, err := a.cli.ImageBuild(a.ctx, buildContext, opts)
    utils.CheckError(err)
    defer res.Body.Close()

    _, err = ioutil.ReadAll(res.Body) // Blocks until the image is built
    utils.CheckError(err)
}

// Remove the image by the given name from the docker daemon.
func (a *Amoeba) removeImage() {
    options     := types.ImageListOptions{All: true}
    images, err := a.cli.ImageList(a.ctx, options)
    utils.CheckError(err)

    for _, image := range images {
        if image.Labels[amoebaBuild] == a.bid {
            opts := types.ImageRemoveOptions{
                Force: true,
                PruneChildren: true,
            }

            _, err := a.cli.ImageRemove(a.ctx, image.ID, opts)
            utils.CheckError(err)
        }
    }
}

func (a *Amoeba) setupClients(clients []string) {
    var wg sync.WaitGroup

    path := filepath.Join(a.dir, a.bid)
    name := strings.Split(a.bid, "-")[0]

    for i, url := range clients {
        wg.Add(1)
        go func() {
            defer wg.Done()
            repoPath := filepath.Join(path, "client" + strconv.Itoa(i))

            repo.Clone(url, "", repoPath)
            repo.OverrideCompose(repoPath, name, a.bid)
        }()
    }

    wg.Wait()
}

func (a *Amoeba) startClients(clients []string) ([]Output) {
    var outputs []Output

    path := filepath.Join(a.dir, a.bid)

    for i, url := range clients {
        repoPath    := filepath.Join(path, "client" + strconv.Itoa(i))
        cmd, output := dockerComposeUp(a.bid, repo.ParseName(url), repoPath)

        outputs = append(outputs, output)
        a.cmds  = append(a.cmds, cmd)
    }

    return outputs
}

func dockerComposeUp(id, repo, dir string) (*exec.Cmd, Output) {
    output := Output{}
    output.Name = repo

    cmd := exec.Command("docker-compose", "up", "--abort-on-container-exit")
    cmd.Dir = dir

    stdout, err := cmd.StdoutPipe()
    utils.CheckError(err)
    stderr, err := cmd.StderrPipe()
    utils.CheckError(err)

    output.Stdout = stdout
    output.Stderr = stderr

    err = cmd.Start()
    utils.CheckError(err)

    return cmd, output
}

func dockerComposeDown(dir string) error {
    cmd := exec.Command("docker-compose","down", "--remove-orphans")
    cmd.Dir = dir

    return cmd.Run()
}
