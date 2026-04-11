// Package controller implements the web UI HTTP handlers for managing I2P
// tunnel configuration, lifecycle control, and status monitoring.
package controller

import (
	"net/http"
	"path"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

func handler(r *http.Request) string {
	if r != nil {
		dir, file := path.Split(r.URL.Path)
		if dir == "/" {
			if file == "home" {
				return "group"
			}
			if file == "new" {
				return "new"
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
		dir, file := path.Split(r.URL.Path)
		if file == "config" || file == "control" {
			// Extract tunnel name: the path segment immediately before the action.
			// For /my-tunnel/control → "my-tunnel"
			// For /group/my-tunnel/control → "my-tunnel"
			tunnelName := path.Base(dir)
			return i2ptunnel.Clean(tunnelName)
		}
	}
	return ""
}
