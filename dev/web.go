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
    u := url.URL{Scheme: "ws", Host: *addr, Path: "/build/dummy-ed59cc75335f869d2378a79924332f17ca1beffa/dummy/stdout"}

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
