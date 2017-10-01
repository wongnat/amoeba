package main

import (
    "io"
    "os"
    "log"
    "sync"
    "strconv"
    "net/http"
    "encoding/json"
    "amoeba/amoeba"
    "amoeba/utils"
    "github.com/gorilla/websocket"
    "github.com/gorilla/mux"
)

const defaultMaxBuilds = 8

var builds *outputMap  // global adt to store/serve amoeba.Output
var maxBuilds int64    // maximmum number of bilds that can occur
var count int64        // current number of active builds
var countMu sync.Mutex // mutex to control updating/testing count

var upgrader = websocket.Upgrader{} // upgrade http conn to websocket

func main() {
    args   := os.Args
    length := len(args)

    if length < 2 || length > 3 {
        log.Fatal("Error: unexpected number of arguments")
    }

    if length == 3 {
        num, err := strconv.ParseInt(args[2], 10, 64)
        utils.CheckError(err)
        maxBuilds = num
    } else {
        maxBuilds = defaultMaxBuilds
    }

    builds = newOutputMap()

    router := mux.NewRouter()
    router.HandleFunc("/build", handleBuild)
    router.HandleFunc("/build/ids", handleIds)
    router.HandleFunc("/build/{id}/clients", handleClients)
    router.HandleFunc("/build/{id}/{client}/stdout", handleOutput)

    port := args[1]
    log.Println("Amoeba server listening on port " + port + " ...")
    http.ListenAndServe(":" + port, router)
}

// Serves json list of build ids
func handleIds(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        w.WriteHeader(http.StatusBadRequest)
        return
    }

    bids := builds.TopKeys()
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(bids)
}

// Serves json list of client names given the build id
func handleClients(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        w.WriteHeader(http.StatusBadRequest)
        return
    }

    vars := mux.Vars(r)
    id   := vars["id"]

    clients := builds.BotKeys(id)
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(clients)
}

// Serves websocket of docker-compose stdout for client repo.
func handleOutput(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    id   := vars["id"]
    cli  := vars["client"]

    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    defer conn.Close()

    out, ok := builds.Load(id, cli)
    if !ok {
        return
    }

    copy(conn, out.Stdout)
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

    name := repository["name"].(string)
    sha  := headCommit["id"].(string)
    url  := repository["ssh_url"].(string)
    bid  := name + "-" + sha

    log.Println("Received request to test: " + bid)

    a, err := amoeba.NewAmoeba(url, bid, "./server/builds")
    if err != nil {
        w.Header().Set("Content-Type", "text/plain")
        w.WriteHeader(http.StatusInternalServerError)
        io.WriteString(w, err.Error())
        return
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

    val := make(map[string]amoeba.Output)
    for _, output := range outputs {
        val[output.Name] = output
    }

    builds.Insert(bid, val)
    errs := a.Wait()

    passed := true
    for _, err = range errs {
        if err != nil {
            passed = false
            break
        }
    }

    w.Header().Set("Content-Type", "text/plain")
    w.WriteHeader(http.StatusOK)
    if passed {
        io.WriteString(w, "PASSED")
    } else {
        io.WriteString(w, "FAILED")
    }

    countMu.Lock()
    count--
    countMu.Unlock()

    log.Println("Finished testing: " + bid)
}
