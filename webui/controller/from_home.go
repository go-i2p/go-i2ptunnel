// Package controller implements the web UI HTTP handlers for managing I2P
// tunnel configuration, lifecycle control, and status monitoring.
package controller

import (
	"net/http"
	"path"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

func handler(r *http.Request) string {
	if r == nil {
		return "group"
	}
	return routeFromPath(r.URL.Path)
}

// routeFromPath returns the route name for a URL path.
func routeFromPath(urlPath string) string {
	dir, file := path.Split(urlPath)
	if dir == "/" {
		return routeRootPath(file)
	}
	return routeSubPath(file)
}

func routeRootPath(file string) string {
	if file == "home" {
		return "group"
	}
	if file == "new" {
		return "new"
	}
	return "group"
}

func routeSubPath(file string) string {
	switch file {
	case "config":
		return "config"
	case "control":
		return "control"
	default:
		return "group"
	}
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
