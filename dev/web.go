package main

import (
    "fmt"
    "flag"
    //"time"
    "net/url"
    "github.com/gorilla/websocket"
)

func main() {
    socket()
    //local()
}

func socket() {
    var addr = flag.String("addr", "localhost:1234", "http service address")
    u := url.URL{Scheme: "ws", Host: *addr, Path: "/build/dummy-cd46ed208331d82c36d5d2ed4e2818d388bf6796/dummy/stdout"}

    conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
    if err != nil {
        panic(err)
    }

    for {
        n, buf, err := conn.ReadMessage()
        if err != nil {
            break
        }

        if n == 0 {
            break
        }

        fmt.Print(string(buf))
    }
}
