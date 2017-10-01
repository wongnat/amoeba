package repo

import (
    "os"
    "strings"
    "os/user"
    "io/ioutil"
    "path/filepath"
    "gopkg.in/yaml.v2"
    "gopkg.in/libgit2/git2go.v24"
    "github.com/jhoonb/archivex"
    "amoeba/utils"
)

// Clones the repo given by the url to the given path.
// If sha is not empty, that commit is checked out.
func Clone(url, sha, path string) {
    cbs := git.RemoteCallbacks{
        CertificateCheckCallback: gitCert,
        CredentialsCallback: gitCred,
    }

    cloneOpts := &git.CloneOptions{}
    cloneOpts.FetchOptions = &git.FetchOptions{}
    cloneOpts.CheckoutOpts = &git.CheckoutOpts{}
    cloneOpts.CheckoutOpts.Strategy = 1
    cloneOpts.FetchOptions.RemoteCallbacks = cbs

    repo, err := git.Clone(url, path, cloneOpts)
    utils.CheckError(err)

    if sha != "" {
        oid, err := git.NewOid(sha)
        utils.CheckError(err)

        commit, err := repo.LookupCommit(oid)
        utils.CheckError(err)

        tree, err := commit.Tree()
        utils.CheckError(err)

        opts := git.CheckoutOpts{Strategy: git.CheckoutSafe}
        err = repo.CheckoutTree(tree, &opts)
        utils.CheckError(err)
    }
}

// Returns the newly created tar archive of the repo at the given path.
func Archive(path string) *os.File {
    tar := new(archivex.TarFile)
    defer tar.Close()

    tar.Create(path)
    tar.AddAll(path, false)

    archive, err := os.Open(path + ".tar")
    utils.CheckError(err)

    return archive
}

// Generate docker-compose.override.yml file in the given dir.
func OverrideCompose(dir, repo, image string) {
    m := make(map[string]interface{})

    in, err := ioutil.ReadFile(filepath.Join(dir, "docker-compose.yml"))
    utils.CheckError(err)

    err = yaml.Unmarshal(in, &m)
    utils.CheckError(err)

    for k := range m {
        if k == repo {
            values := m[repo].(map[interface{}]interface{})
            values["image"] = image
            break
        } else if k == "services" {
            services := m["services"].(map[interface{}]interface{})
            values   := services[repo].(map[interface{}]interface{})
            if values != nil {
                values["image"] = image
            }

            break
        }
    }

    out, err := yaml.Marshal(&m)
    utils.CheckError(err)

    file, err := os.Create(filepath.Join(dir, "docker-compose.override.yml"))
    utils.CheckError(err)

    _, err = file.Write(out)
    utils.CheckError(err)

    err = file.Sync()
    utils.CheckError(err)

    err = file.Close()
    utils.CheckError(err)
}

// Format of amoeba.yml file
type config struct {
    Clients []string `yaml:"clients"`
}

// Return list of client services' github ssh urls from amoeba.yml in dir.
func ParseConfig(dir string) []string {
    var clients []string

    c := config{}

    file, err := ioutil.ReadFile(filepath.Join(dir, "amoeba.yml"))
    utils.CheckError(err)

    err = yaml.Unmarshal(file, &c)
    utils.CheckError(err)

    for _, client := range c.Clients {
        clients = append(clients, client)
    }

    return clients
}

// Return the name of the repo given by the specified git ssh url.
func ParseName(url string) string {
    temp := strings.Split(url, "/")
    return strings.Split(temp[len(temp) - 1], ".")[0]
}

// Callback to generate git ssh credentials.
func gitCred(url, username string, t git.CredType) (git.ErrorCode, *git.Cred) {
    usr, err := user.Current()
    utils.CheckError(err)

    sshPath := filepath.Join(usr.HomeDir, ".ssh")
    pubPath := filepath.Join(sshPath, "id_rsa.pub")
    keyPath := filepath.Join(sshPath, "id_rsa")

    ret, cred := git.NewCredSshKey("git", pubPath, keyPath, "")
    return git.ErrorCode(ret), &cred
}

// Callback to validate certificates
func gitCert(cert *git.Certificate, valid bool, host string) git.ErrorCode {
    return 0
}
