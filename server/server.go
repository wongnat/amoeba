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

type outputMap {
    Mut sync.Mutex
    Map map[string]amoeba.Output
}

var builds map[string]outputMap

var count int64 = 0
var maxBuilds int64

var buildsMut sync.Mutex
var countMut  sync.Mutex

func main() {
    args   := os.Args
    length := len(args)

    if length < 2 || length > 3 {
        log.Fatal("Error: unexpected number of arguments")
    }

    if length == 3 {
        maxBuilds, err := strconv.ParseInt(args[2], 10, 64)
        utils.CheckError(err)
    } else {
        maxBuilds = defaultMaxBuilds
    }

    builds = make(map[string]outputMap)

    router := mux.NewRouter()
    router.HandleFunc("/", handleRoot)
    router.HandleFunc("/build", handleBuild)
    router.HandleFunc("/build/ids", handleIds)
    router.HandleFunc("/build/{id}/clients", handleClients)
    router.HandleFunc("/build/{id}/{client}/{output}", handleOutput)

    port := args[1]
    log.Println("Amoeba server listening on port " + port + " ...")
    http.ListenAndServe(":" + port, router)
}

func handleRoot(w http.ResponseWriter, r *http.Request) {

}

func handleIds(w http.ResponseWriter, r *http.Request) {

}

func handleClients(w http.ResponseWriter, r *http.Request) {

}

// Serves websocket of docker-compose output.
func handleOutput(w http.ResponseWriter, r *http.Request) {
    var upgrader = websocket.Upgrader{}

    vars := mux.Vars(r)
    id   := vars["iD"]
    cli  := vars["client"]
    out  := vars["output"]

    // TODO check vars are legal

    path := filepath.Join(logsDir, id, cli)
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    defer conn.Close()

    buildsMut.Lock()
    outputMap := builds[id]
    if outputMap != nil {
        outputMap.Mut.Lock()
        buildsMut.Unlock()
        output := outputMap.Map[cli]
        if output != nil {
            outputMap.Mut.Unlock()
            if out == "stdout" {
                copy(conn, output.Stdout)
            } else {
                copy(conn, output.Stderr)
            }
        } else {
            outputMap.Mut.Unlock()
        }
    } else {
        buildsMut.Unlock()
        if _, err := os.Stat(path); os.IsNotExist(err) {
            w.WriteHeader(http.StatusNotFound)
        } else {
            file, err := os.Open(filepath.Join(path, out))
            if err != nil {
                w.WriteHeader(http.StatusNotFound)
                return
            }

            copy(conn, file)
            defer file.Close()
        }

        return
    }

    buildsMut.Lock()
    delete(builds, buildID)
    buildsMut.Unlock()

    // atomic.AddInt64(&count, -1)
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
        countMut.Lock()

        if count < maxBuilds {
            count++
            countMut.Unlock()
            break
        }

        countMut.Unlock()
    }

    outputs := a.Start()

    val := outputMap{}
    for _, output := range outputs {
        val[output.Name] = output
    }

    buildsMut.Lock()
    builds[buildID] = val
    buildsMut.Unlock()

    errs := a.Wait()

    isntError := true
    for _, err = range errs {
        if err != nil {
            io.WriteString(w, "You broke at least one client repo\n")
            isntError = false
            break
        }
    }

    if isntError {
        io.WriteString(w, "You didn't break anything! Yay!\n")
    }

    countMut.Lock()
    count--
    countMut.Unlock()

    log.Println("Finished testing: " + buildID)
}

// Writes the contents of the given reader to the given websocket.
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
