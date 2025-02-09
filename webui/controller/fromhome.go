package controller

import (
	"net/http"
	"path/filepath"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

func handler(r *http.Request) string {
	if r != nil {
		dir, file := filepath.Split(r.URL.Path)
		if dir == "/" {
			if file == "home" {
				return "group"
			}
		} else {
			if file == "config" {
				return "config"
			} else if file == "control" {
				return "control"
			}
		}
	}
	return "group"
}

func tunnel(r *http.Request) string {
	if r != nil {
		dir, file := filepath.Split(r.URL.Path)
		if file == "config" || file == "control" {
			return i2ptunnel.Clean(dir)
		}
	}
	return ""
}
