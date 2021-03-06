package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

func main() {
	fmt.Printf("Starting scout\n")

	for _, port := range os.Args[1:] {
		// TODO argument validation
		ln, err := net.Listen("tcp", port)
		if err != nil {
			fmt.Fprintf(os.Stderr,
				"Failed to listen to port %s: %s\n", port, err)
			continue
		}
		fmt.Printf("Listening for connections on port %s\n", port)
		conn, err := ln.Accept()
		if err != nil {
			fmt.Fprintf(os.Stderr,
				"Failed to accept traffic on port %s: %s\n", port, err)
			ln.Close()
			continue
		}

		var msg []byte = make([]byte, 5)
		_, err = bufio.NewReader(conn).Read(msg)
		if err != nil {
			fmt.Fprintf(os.Stderr,
				"Failed to read from connection on port %s: %s\n", port, err)
			conn.Close()
			ln.Close()
			continue
		}
		fmt.Printf("\tmessage receieved on port %s from %s: %s\n",
			port, conn.RemoteAddr(), string(msg))

		_, err = fmt.Fprintf(conn, "pong")
		if err != nil {
			fmt.Fprintf(os.Stderr,
				"Failed to send message on port %s: %s\n", port, err)
			conn.Close()
			ln.Close()
			continue
		}

		conn.Close()
		ln.Close()
	}

	fmt.Printf("Scouting complete!\n")
}
