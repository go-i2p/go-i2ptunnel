package controller

import (
	"net/http"
	"os"
	"path/filepath"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
	templates "github.com/go-i2p/go-i2ptunnel/webui/templates"
)

type ControllerGroup struct {
	I2PTunnels []Controller
}

func (cg *ControllerGroup) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cg.HandleHTMLHeader(r, w)
	defer cg.HandleHTMLFooter(r, w)
	switch handler(r) {
	case "group":
		cg.HandleGroup(r, w)
	case "control":
		for _, controller := range cg.I2PTunnels {
			if i2ptunnel.Clean(controller.Name()) == tunnel(r) {
				controller.ServeHTTP(w, r)
				return
			}
		}
		cg.HandleError(r, w)
	case "config":
		for _, controller := range cg.I2PTunnels {
			if i2ptunnel.Clean(controller.Name()) == tunnel(r) {
				controller.Config.ServeHTTP(w, r)
				return
			}
		}
		cg.HandleError(r, w)
	default:
		cg.HandleGroup(r, w)
	}
}

func (cg *ControllerGroup) HandleHTMLHeader(r *http.Request, w http.ResponseWriter) {
	templates.HeaderTemplate.Execute(w, nil)
}

func (cg *ControllerGroup) HandleHTMLFooter(r *http.Request, w http.ResponseWriter) {
	templates.FooterTemplate.Execute(w, nil)
}

func (cg *ControllerGroup) HandleGroup(r *http.Request, w http.ResponseWriter) {
	for _, controller := range cg.I2PTunnels {
		controller.MiniServeHTTP(w, r)
	}
}

func (cg *ControllerGroup) HandleError(r *http.Request, w http.ResponseWriter) {
	r.Form = nil
	// just redirect back to /home
	http.Redirect(w, r, "/home", 302)
}

func NewControllerGroup(directory string) (*ControllerGroup, error) {
	files, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}

	group := &ControllerGroup{
		I2PTunnels: make([]Controller, 0),
	}

	for _, file := range files {
		if !file.IsDir() {
			controller, err := NewController(filepath.Join(directory, file.Name()))
			if err != nil {
				return nil, err
			}
			group.I2PTunnels = append(group.I2PTunnels, *controller)
		}
	}

	return group, nil
}
