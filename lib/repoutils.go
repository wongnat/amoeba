package amoeba

import (
	"github.com/jhoonb/archivex"
	"gopkg.in/libgit2/git2go.v24"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// Clones the repo given by the url to the given path.
// If sha is not empty, that commit is checked out.
func cloneRepo(url, sha, path string) error {
	cbs := git.RemoteCallbacks{
		CertificateCheckCallback: gitCert,
		CredentialsCallback:      gitCred,
	}

	cloneOpts := &git.CloneOptions{}
	cloneOpts.FetchOptions = &git.FetchOptions{}
	cloneOpts.CheckoutOpts = &git.CheckoutOpts{}
	cloneOpts.CheckoutOpts.Strategy = 1
	cloneOpts.FetchOptions.RemoteCallbacks = cbs

	repo, err := git.Clone(url, path, cloneOpts)
	if err != nil {
		return err
	}

	if sha != "" {
		oid, err := git.NewOid(sha)
		if err != nil {
			return err
		}

		commit, err := repo.LookupCommit(oid)
		if err != nil {
			return err
		}

		tree, err := commit.Tree()
		if err != nil {
			return err
		}

		opts := git.CheckoutOpts{Strategy: git.CheckoutSafe}
		err = repo.CheckoutTree(tree, &opts)
		if err != nil {
			return err
		}
	}
}

// Returns the newly created tar archive of the repo at the given path.
func archiveRepo(path string) (*os.File, error) {
	tar := new(archivex.TarFile)
	defer tar.Close()

	tar.Create(path)
	tar.AddAll(path, false)

	archive, err := os.Open(path + ".tar")
	if err != nil {
		return nil, err
	}

	return archive, nil
}

// Generate docker-compose.override.yml file in the given dir.
func overrideCompose(dir, repo, image string) error {
	m := make(map[string]interface{})

	in, err := ioutil.ReadFile(filepath.Join(dir, "docker-compose.yml"))
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(in, &m)
	if err != nil {
		return err
	}

	for k := range m {
		if k == repo {
			values := m[repo].(map[interface{}]interface{})
			values["image"] = image

			break
		} else if k == "services" {
			services := m["services"].(map[interface{}]interface{})
			values := services[repo].(map[interface{}]interface{})
			if values != nil {
				values["image"] = image
			}

			break
		}
	}

	out, err := yaml.Marshal(&m)
	if err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(dir, "docker-compose.override.yml"))
	if err != nil {
		return err
	}

	_, err = file.Write(out)
	if err != nil {
		return err
	}

	err = file.Sync()
	if err != nil {
		return err
	}

	err = file.Close()
	if err != nil {
		return err
	}
}

// Format of amoeba.yml file
type amoebaConfig struct {
	Clients []string `yaml:"clients"`
}

// Return list of client services' github ssh urls from amoeba.yml in dir.
func parseConfig(dir string) ([]string, error) {
	var clients []string

	ac := amoebaConfig{}

	file, err := ioutil.ReadFile(filepath.Join(dir, "amoeba.yml"))
	if err != nil {
		return clients, err
	}

	err = yaml.Unmarshal(file, &ac)
	if err != nil {
		return clients, err
	}

	for _, client := range ac.Clients {
		clients = append(clients, client)
	}

	return clients, nil
}

// Return the name of the repo given by the specified git ssh url.
func parseRepoName(url string) string {
	temp := strings.Split(url, "/")
	return strings.Split(temp[len(temp)-1], ".")[0]
}

// Callback to generate git ssh credentials.
func gitCred(url, username string, t git.CredType) (git.ErrorCode, *git.Cred) {
	var sshPath string
	var pubPath string
	var keyPath string

	usr, err := user.Current()
	if err != nil { // will trigger a git error
		sshPath = ""
		pubPath = ""
		keyPath = ""
	} else {
		sshPath = filepath.Join(usr.HomeDir, ".ssh")
		pubPath = filepath.Join(sshPath, "id_rsa.pub")
		keyPath = filepath.Join(sshPath, "id_rsa")
	}

	ret, cred := git.NewCredSshKey("git", pubPath, keyPath, "")
	return git.ErrorCode(ret), &cred
}

// Callback to validate certificates
func gitCert(cert *git.Certificate, valid bool, host string) git.ErrorCode {
	return 0
}
