package main

import (
    "os"
    "io"
    "io/ioutil"
    "fmt"
    "net/http"
    "amoeba/amoeba"
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
    a, err := amoeba.NewAmoeba("git@github.com:wongnat/dummy.git", "dummy-ed59cc75335f869d2378a79924332f17ca1beffa")
    if err != nil {
        panic(err)
    }
    defer a.Close()

    outputs := a.Start()
    go io.Copy(os.Stdout, outputs[0].Stdout)
    a.Wait()
}
