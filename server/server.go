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
var buildCount int64 = 0
var mu sync.Mutex

var buildsMu sync.Mutex

var builds map[string][]amoeba.Stream
var upgrader = websocket.Upgrader{}

func handleOutput(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    path := filepath.Join("./out", vars["buildID"], vars["clientRepo"])

    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    defer conn.Close()

    //buildsMu.Lock()
    streams := builds[vars["buildID"]]

    if streams == nil {
        //buildsMu.Unlock()
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
    for _, stream := range streams {
        if stream.Name == vars["clientRepo"] {
            if vars["output"] == "stdout" {
                //buildsMu.Unlock()
                copy(conn, stream.Stdout)
                break
            }
        }
    }
}

func copy(conn *websocket.Conn, r io.Reader) error {
    log.Println("copying bytes to websocket")
    buf := make([]byte, 1024)
    for {
        n, err := r.Read(buf)
        //log.Println(string(buf))
        if err != nil && err != io.EOF {
            return err
        }

        if n == 0 {
            break
        }

        err = conn.WriteMessage(websocket.TextMessage, buf[:n])
        if err != nil {
            //return err
        }
    }

    return nil
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

    cmds, streams := a.StartDeploy(sshURL, group)
    buildsMu.Lock()
    builds[group] = streams
    buildsMu.Unlock()
    errs := a.WaitDeploy(cmds)

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
    buildsMu.Lock()
    delete(builds, group)
    buildsMu.Unlock()
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

    builds = make(map[string][]amoeba.Stream)
    router := mux.NewRouter()

    router.HandleFunc("/build", handleBuild)
    router.HandleFunc("/build/{buildID}/{clientRepo}/{output}", handleOutput)

    port := args[1]
    log.Println("Amoeba listening on port " + port + " ...")
    http.ListenAndServe(":" + port, router)
}
