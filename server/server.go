package main

import (
    "io"
    "os"
    "log"
    "sync"
    "strconv"
    "net/http"
    "sync/atomic"
    "path/filepath"
    "encoding/json"
    "amoeba/amoeba"
    "amoeba/utils"
    "github.com/gorilla/websocket"
    "github.com/gorilla/mux"
)

const defaultMaxBuilds = 4

var maxBuilds int64
//var builds sync.Map
var buildCount int64 = 0
var mu sync.Mutex

var upgrader = websocket.Upgrader{}

func handleIndex(w http.ResponseWriter, r *http.Request) {
    // TODO
    if r.Method != http.MethodGet {
        w.WriteHeader(http.StatusBadRequest)
        return
    }
}

func handleOutput(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    path := filepath.Join("./out", vars["buildID"], vars["clientRepo"])
    if _, err := os.Stat(path); os.IsNotExist(err) {
        w.WriteHeader(http.StatusNotFound)
        return
    }

    file, err := os.Open(filepath.Join(path, vars["output"]))
    if err != nil {
        return
    }

    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Fatal("Error: could not setup websocket")
        return
    }
    defer conn.Close()

    // TODO this doesn't work right
    buf := make([]byte, 1024)
    done := false
    for {
        _, err = file.Read(buf)
        if err != nil {
            done = true
        }

        err = conn.WriteMessage(websocket.TextMessage, buf)
        if done {
            break
        }
    }
}

// Intended as a github push/pr event.
func handleBuild(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        w.WriteHeader(http.StatusBadRequest)
        return
    }

    a, err := amoeba.NewAmoeba()
    if err != nil {
        panic(err)
    }
    defer a.Close()

    var jsonIn map[string]interface{}
    dec := json.NewDecoder(r.Body)
    dec.Decode(&jsonIn)

    headCommit := jsonIn["head_commit"].(map[string]interface{})
    repository := jsonIn["repository"].(map[string]interface{})

    repoName := repository["name"].(string)
    commitID := headCommit["id"].(string)
    sshURL   := repository["ssh_url"].(string)

    group := repoName + "-" + commitID
    log.Println("Received request to test: " + group)

    for {
        mu.Lock()

        if atomic.LoadInt64(&buildCount) < maxBuilds {
            atomic.AddInt64(&buildCount, 1)
            mu.Unlock()
            break // Begin building
        }

        mu.Unlock()
    }

    errs := a.StartDeploy(sshURL, group)

    isntError := true
    for _, err = range errs {
        if err != nil {
            io.WriteString(w, "You broke at least one client repo\n")
            isntError = false
        }
    }

    if isntError {
        io.WriteString(w, "You didn't break anything! Yay!\n")
    }

    a.TearDownDeploy(group)

    log.Println("Finished testing: " + group)

    atomic.AddInt64(&buildCount, -1)
}

func main() {
    var err error

    args := os.Args
    length := len(args)

    if length < 2 || length > 3 {
        log.Fatal("Error: unexpected number of arguments")
    }

    if length == 3 {
        maxBuilds, err = strconv.ParseInt(args[2], 10, 64)
        utils.CheckError(err)
    } else {
        maxBuilds = defaultMaxBuilds
    }

    log.Println("Maximum number of builds: " + strconv.FormatInt(maxBuilds, 10))

    router := mux.NewRouter()

    router.HandleFunc("/", handleIndex)
    router.HandleFunc("/build", handleBuild)
    router.HandleFunc("/build/{buildID}/{clientRepo}/{output}", handleOutput)

    port := args[1]
    log.Println("Amoeba listening on port " + port + " ...")
    http.ListenAndServe(":" + port, router)
}
