package main

import (
    //"os"
    //"io"
    "fmt"
    "flag"
    "net/url"
    //"io/ioutil"
    //"fmt"
    //"net/http"
    "github.com/gorilla/websocket"
)

func main() {
    socket()
    //local()
}

func socket() {
    var addr = flag.String("addr", "localhost:1234", "http service address")
    u := url.URL{Scheme: "ws", Host: *addr, Path: "/build/dummy-ed59cc75335f869d2378a79924332f17ca1beffa/dummy/stdout"}
    // dial web socket
    conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
    if err != nil {
        panic(err)
    }
    // _, r, err := sock.NextReader()
    // if err != nil {
    //     panic(err)
    // }
    // io.Copy(os.Stdout, r)
    buf := make([]byte, 1024)
    done := false
    for {

        _, buf, err = conn.ReadMessage()
        if err != nil {
            done = true
        }
        fmt.Print(string(buf))
        if done {
            break
        }
    }
}
