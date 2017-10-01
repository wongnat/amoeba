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

const defaultMaxBuilds = 8

var builds outputMap   // global adt to store/serve amoeba.Output
var maxBuilds int      // maximmum number of bilds that can occur
var count int          // current number of active builds
var countMu sync.Mutex // mutex to control updating/testing count

var upgrader = websocket.Upgrader{} // upgrade http conn to websocket

func main() {
    args   := os.Args
    length := len(args)

    if length < 2 || length > 3 {
        log.Fatal("Error: unexpected number of arguments")
    }

    if length == 3 {
        maxBuilds, err := strconv.ParseInt(args[2], 10, 0)
        utils.CheckError(err)
    } else {
        maxBuilds = defaultMaxBuilds
    }

    builds = newOutputMap()

    router := mux.NewRouter()
    router.HandleFunc("/build", handleBuild) // post only
    router.HandleFunc("/build/ids", handleIds) // json
    router.HandleFunc("/build/{id}/clients", handleClients) // json
    router.HandleFunc("/build/{id}/{client}/stdout", handleOutput) // websocket, plain text

    port := args[1]
    log.Println("Amoeba server listening on port " + port + " ...")
    http.ListenAndServe(":" + port, router)
}

func handleIds(w http.ResponseWriter, r *http.Request) {
    buildIds := builds.TopKeys()
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOk)
    json.NewEncoder(w).Encode(buildIds)
}

func handleClients(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    id   := vars["id"]

    clients := builds.BotKeys(id)
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOk)
    json.NewEncoder(w).Encode(clients)
}

// Serves websocket of docker-compose stdout.
func handleOutput(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    id   := vars["id"]
    cli  := vars["client"]

    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    defer conn.Close()

    out := builds.Load(id, cli)
    if out == nil {
        return
    }

    copy(conn, output.Stdout)
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

    a, err := amoeba.NewAmoeba(sshURL, buildID, "./server/builds")
    if err != nil {
        panic(err)
    }
    defer a.Close()

    for { // Block until the number of in-progress builds is < maxBuilds
        countMu.Lock()
        if count < maxBuilds {
            count++
            countMu.Unlock()
            break
        }
        countMu.Unlock()
    }

    outputs := a.Start()

    // Build an inner map for our outputMap
    val := make(map[string]amoeba.Output)
    for _, output := range outputs {
        val[output.Name] = output
    }

    builds.Insert(buildID, val)
    errs := a.Wait()

    // Look for errors
    passed := true
    for _, err = range errs {
        if err != nil {
            passed = false
            break
        }
    }

    if passed {
        io.WriteString(w, "You didn't break anything! Yay!\n")
    } else {
        io.WriteString(w, "You broke at least one client repo\n")
    }

    countMu.Lock()
    count--
    countMu.Unlock()

    log.Println("Finished testing: " + buildID)
}
