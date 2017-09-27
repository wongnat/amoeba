package repo

import (
    "os"
    "strings"
    "os/user"
    "encoding/json"
    "path/filepath"
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

// Generate docker-compose.override.yml file at the given path.
// TODO: This function is not finalized
func OverrideCompose(path, repo, image string) {
    file, err := os.Create(filepath.Join(path, "docker-compose.override.yml"))
    utils.CheckError(err)

    _, err = file.WriteString(repo + ":\n    image: \"" + image + "\"\n")
    utils.CheckError(err)

    err = file.Sync()
    utils.CheckError(err)

    err = file.Close()
    utils.CheckError(err)
}

// Return list of client services github urls from amoeba.json file.
// TODO: This function is not finalized
func ParseConfig(path string) []string {
    var jsonIn map[string]interface{}
    var clients []string

    file, err := os.Open(filepath.Join(path, "amoeba.json"))
    utils.CheckError(err)

    dec := json.NewDecoder(file)
    dec.Decode(&jsonIn)

    tmpClients := jsonIn["clients"].([]interface{})
    for _, c := range tmpClients {
        clients = append(clients, c.(string))
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
// TODO: this function is not finalized
func gitCert(cert *git.Certificate, valid bool, host string) git.ErrorCode {
    return 0
}
