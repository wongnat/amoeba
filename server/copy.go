package main

import (
	"github.com/gorilla/websocket"
	"io"
)

// Writes the contents of the given reader to the given websocket conn.
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
