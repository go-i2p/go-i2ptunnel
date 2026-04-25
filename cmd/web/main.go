package main

import (
	"flag"
	"log"
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
		log.Fatalf("failed to listen on %s: %v", addr, err)
	}
	controllerGroup, err := controller.NewControllerGroup(*configDir)
	if err != nil {
		log.Fatalf("failed to initialize controller group from %q: %v", *configDir, err)
	}
	if err := http.Serve(ln, controllerGroup); err != nil {
		log.Fatalf("http server error: %v", err)
	}
}
