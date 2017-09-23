package main

import (
    "os"
    "io/ioutil"
    "fmt"
    "net/http"
    "amoeba/amoeba"
    //"github.com/gorilla/websocket"
)

func main() {
    request()
    //local()
}

func request() {
    file, err := os.Open("./dev/example.json")
    if err != nil {
        panic(err)
    }
    defer file.Close()

    resp, err := http.Post("http://localhost:1234/build", "application/json", file)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)
    fmt.Println(string(body))
}

func local() {
    a, err := amoeba.NewAmoeba()
    if err != nil {
        panic(err)
    }
    defer a.Close()

    a.StartDeploy("git@github.com:wongnat/dummy.git", "dummy-ed59cc75335f869d2378a79924332f17ca1beffa")

    a.TearDownDeploy("dummy-ed59cc75335f869d2378a79924332f17ca1beffa")
}
