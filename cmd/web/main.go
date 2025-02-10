package main

import (
	"flag"
	"net"
	"net/http"
	"strconv"

	"github.com/go-i2p/go-i2ptunnel/webui/controller"
)

func main() {
	configDir := flag.String("config", "", "Path to the config directory")
	host := flag.String("host", "localhost", "Host to listen on")
	port := flag.Int("port", 8089, "Port to listen on")
	flag.Parse()
	addr := net.JoinHostPort(*host, strconv.Itoa(*port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	controllerGroup, err := controller.NewControllerGroup(*configDir)
	if err != nil {
		panic(err)
	}
	if err := http.Serve(ln, controllerGroup); err != nil {
		panic(err)
	}
}
