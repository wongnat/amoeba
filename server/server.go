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

const (
    logsDir = "./logs"
    defaultMaxBuilds = 4
)

var builds map[string][]amoeba.Output

var buildsCount int64 = 0
var maxBuilds int64

var buildsMu sync.Mutex
var countMu sync.Mutex

func handleOutput(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)

    // TODO check vars are legal

    path := filepath.Join(logsDir, vars["buildID"], vars["clientRepo"])

    var upgrader = websocket.Upgrader{}
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    defer conn.Close()

    outputs := builds[vars["buildID"]]

    if outputs == nil {

        log.Println("live build not available")
        if _, err := os.Stat(path); os.IsNotExist(err) {
            w.WriteHeader(http.StatusNotFound)
        } else {
            file, err := os.Open(filepath.Join(path, vars["output"]))
            if err != nil {
                w.WriteHeader(http.StatusNotFound)
                return
            }
            copy(conn, file)
            defer file.Close()
        }

        return
    }

    log.Println("Getting live build")
    for _, output := range outputs {
        if output.Name == vars["clientRepo"] {
            if vars["output"] == "stdout" {
                copy(conn, output.Stdout)
            } else {
                copy(conn, output.Stderr)
            }

            break
        }
    }

    buildsMu.Lock()
    delete(builds, buildID)
    buildsMu.Unlock()

    atomic.AddInt64(&buildsCount, -1)
}


// Intended as a github push/pr event.
func handleBuild(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        w.WriteHeader(http.StatusBadRequest)
        return
    }

    var jsonIn map[string]interface{}

    dec := json.NewDecoder(r.Body)
    dec.Decode(&jsonIn)

    headCommit := jsonIn["head_commit"].(map[string]interface{})
    repository := jsonIn["repository"].(map[string]interface{})

    repoName := repository["name"].(string)
    commitID := headCommit["id"].(string)
    sshURL   := repository["ssh_url"].(string)

    buildID := repoName + "-" + commitID
    log.Println("Received request to test: " + buildID)

    a, err := amoeba.NewAmoeba(sshURL, buildID)
    if err != nil {
        panic(err)
    }
    defer a.Close()

    for {
        countMu.Lock()

        if atomic.LoadInt64(&buildsCount) < maxBuilds {
            atomic.AddInt64(&buildsCount, 1)
            countMu.Unlock()
            break // Begin building
        }

        countMu.Unlock()
    }

    outputs := a.Start()

    buildsMu.Lock()
    builds[buildID] = outputs
    buildsMu.Unlock()

    errs := a.Wait()

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

    log.Println("Finished testing: " + buildID)

    // buildsMu.Lock()
    // delete(builds, buildID)
    // buildsMu.Unlock()
    //
    // atomic.AddInt64(&buildsCount, -1)
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

    builds = make(map[string][]amoeba.Output)
    router := mux.NewRouter()

    router.HandleFunc("/build", handleBuild)
    router.HandleFunc("/build/{buildID}/{clientRepo}/{output}", handleOutput)

    port := args[1]
    log.Println("Amoeba listening on port " + port + " ...")
    http.ListenAndServe(":" + port, router)
}

func copy(conn *websocket.Conn, r io.Reader) error {
    buf := make([]byte, 1024)
    for {
        n, err := r.Read(buf)
        if err != nil && err != io.EOF {
            return err
        }

        if n == 0 {
            break
        }

        err = conn.WriteMessage(websocket.TextMessage, buf[:n])
        if err != nil {
            return err
        }
    }

    return nil
}
