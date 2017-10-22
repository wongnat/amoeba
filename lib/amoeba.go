package amoeba

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const amoebaBuild = "com.amoeba.build" // Label key for docker images built

// Amoeba ...
type Amoeba struct {
	cli  *client.Client
	ctx  context.Context
	url  string
	bid  string
	dir  string
	cmds []*exec.Cmd
}

// ComposeOutput ...
type ComposeOutput struct {
	RepoName string
	Stdout   io.Reader
	Stderr   io.Reader
}

// NewAmoeba ...
func NewAmoeba(url, sha, dir string) (*Amoeba, error) {
	docker, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	bid := parseRepoName(url) + "-" + sha
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
func (a *Amoeba) Start() ([]ComposeOutput, error) {
	var defaultOutputs []ComposeOutput

	buildPath := filepath.Join(a.dir, a.bid)
	if err := checkDir(a.dir); err != nil {
		return defaultOutputs, err
	}

	if err := checkDir(buildPath); err != nil {
		return defaultOutputs, err
	}

	sha := strings.Split(a.bid, "-")[1]
	dest := filepath.Join(buildPath, "target")

	err := cloneRepo(a.url, sha, dest)
	if err != nil {
		return defaultOutputs, err
	}

	buildContext := archiveRepo(dest)
	defer buildContext.Close()

	// Build the image while setting up client repos
	buildChan := make(chan error)
	go func() {
		buildChan <- a.buildImage(buildContext)
	}()

	clients := parseConfig(dest)
	if err = a.setupClients(clients); err != nil {
		os.RemoveAll(buildPath)
		return defaultOutputs, err
	}

	buildErr := <-buildChan
	if buildErr != nil {
		os.RemoveAll(buildPath)
		return defaultOutputs, buildErr
	}

	outputs, err := a.startClients(clients)
	if err != nil {
		os.RemoveAll(buildPath)
		return defaultOutputs, err
	}

	return outputs, nil
}

// Wait ...
func (a *Amoeba) Wait() ([]error, error) {
	var defaultErrs []error

	errs := make([]error, len(a.cmds))
	for i, cmd := range a.cmds {
		err := cmd.Wait()
		if err != nil {
			errs[i] = err
		}
	}

	buildPath := filepath.Join(a.dir, a.bid)
	dirs, err := ioutil.ReadDir(buildPath)
	if err != nil {
		return defaultErrs, err
	}

	for _, dir := range dirs {
		name := dir.Name()
		if strings.Contains(name, "client") {
			err := dockerComposeDown(filepath.Join(buildPath, name))
			if err != nil {
				return defaultErrs, err
			}
		}
	}

	err = os.RemoveAll(buildPath)
	if err != nil {
		return defaultErrs, err
	}

	err = a.removeImage()
	if err != nil {
		return defaultErrs, err
	}

	return errs, nil
}

// Builds the image from the given buildContext.
// this is good
func (a *Amoeba) buildImage(buildContext io.Reader) error {
	opts := types.ImageBuildOptions{
		Tags:        []string{a.bid + ":latest"},
		Remove:      true,
		ForceRemove: true,
		Labels:      map[string]string{amoebaBuild: a.bid},
	}

	res, err := a.cli.ImageBuild(a.ctx, buildContext, opts)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	_, err = ioutil.ReadAll(res.Body) // Blocks until built
	if err != nil {
		return err
	}
}

// Remove the image by the given name from the docker daemon.
// this is good
func (a *Amoeba) removeImage() error {
	options := types.ImageListOptions{All: true}
	images, err := a.cli.ImageList(a.ctx, options)
	if err != nil {
		return err
	}

	for _, image := range images {
		if image.Labels[amoebaBuild] == a.bid {
			opts := types.ImageRemoveOptions{
				Force:         true,
				PruneChildren: true,
			}

			_, err := a.cli.ImageRemove(a.ctx, image.ID, opts)
			if err != nil {
				return err
			}

			break // we found it
		}
	}
}

func (a *Amoeba) setupClients(clients []string) error {
	var clientSetups sync.WaitGroup

	path := filepath.Join(a.dir, a.bid)
	name := strings.Split(a.bid, "-")[0]
	done := make(chan bool)

	errChan := make(chan error)

	for i, url := range clients {
		clientSetups.Add(1)
		go func() {
			defer clientSetups.Done()

			repoPath := filepath.Join(path, "client"+strconv.Itoa(i))
			err := cloneRepo(url, "", repoPath)
			if err != nil {
				errChan <- err
				return
			}

			err := overrideCompose(repoPath, name, a.bid)
			if err != nil {
				errChan <- err
			}
		}()
	}

	go func() {
		clientSetups.Wait()
		done <- true
	}()

	// Either wait until we're done or return an error
	// on first instance of an error occuring
	select {
	case <-done:
		// success
	case err := <-errChan:
		return err // already know err != nil
	}

	return nil
}

// this is probably fine
func (a *Amoeba) startClients(clients []string) ([]ComposeOutput, error) {
	var defaultOutputs []ComposeOutput

	outputs := make([]ComposeOutput, len(clients)) // pre-alloc, so no resizing
	path := filepath.Join(a.dir, a.bid)

	for i, url := range clients {
		repoPath := filepath.Join(path, "client"+strconv.Itoa(i))
		cmd, output, err := dockerComposeUp(a.bid, parseRepoName(url), repoPath)
		if err != nil {
			return defaultOutputs, err // nil outputs
		}

		outputs[i] = output
		a.cmds = append(a.cmds, cmd)
	}

	return outputs, nil
}

func dockerComposeUp(id, repo, dir string) (*exec.Cmd, ComposeOutput, error) {
	output := ComposeOutput{}
	output.RepoName = repo

	cmd := exec.Command("docker-compose", "up", "--abort-on-container-exit")
	cmd.Dir = dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return cmd, output, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return cmd, output, err
	}

	output.Stdout = stdout
	output.Stderr = stderr

	if err = cmd.Start(); err != nil {
		return cmd, output, err
	}

	return cmd, output, nil
}

func dockerComposeDown(dir string) error {
	cmd := exec.Command("docker-compose", "down", "--remove-orphans")
	cmd.Dir = dir

	return cmd.Run()
}

// Check if the given dir exists and create it if it does not.
func checkDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.Mkdir(dir, os.ModePerm)
	}

	return nil
}
