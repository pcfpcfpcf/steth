package web

import (
	"errors"
	"net/http"
	"path/filepath"

	"github.com/pcfpcfpcf/steth/internal/library"
)

// libraryFile is a model file under the places, with the path it resolves
// to, which is how an engine that resolves links names it.
type libraryFile struct {
	library.File
	Real string `json:"real"`
}

type libraryView struct {
	Places []library.Place `json:"places"`
	Files  []libraryFile   `json:"files"`
}

func (a *Actions) libraryView() libraryView {
	files := library.Scan(a.Places.Paths())
	view := libraryView{Places: a.Places.All(), Files: make([]libraryFile, len(files))}
	for i, f := range files {
		real, err := filepath.EvalSymlinks(f.Path)
		if err != nil {
			real = f.Path
		}
		view.Files[i] = libraryFile{File: f, Real: real}
	}
	return view
}

func (a *Actions) libraryList(w http.ResponseWriter, r *http.Request) {
	reply(w, a.libraryView())
}

// placeRequest names a place to add or remove.
type placeRequest struct {
	Path string `json:"path"`
}

func (a *Actions) placeAdd(w http.ResponseWriter, r *http.Request) {
	a.place(w, r, a.Places.Add)
}

func (a *Actions) placeRemove(w http.ResponseWriter, r *http.Request) {
	a.place(w, r, a.Places.Remove)
}

// place adds or removes one place and answers with the library after it.
func (a *Actions) place(w http.ResponseWriter, r *http.Request, act func(string) error) {
	var req placeRequest
	if err := decode(w, r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if req.Path == "" {
		fail(w, http.StatusBadRequest, errors.New("no path"))
		return
	}
	if err := act(req.Path); err != nil {
		fail(w, http.StatusConflict, err)
		return
	}
	reply(w, a.libraryView())
}

// browse lists one directory of this machine, to pick a place: its
// subdirectories and its model files, nothing else.
func (a *Actions) browse(w http.ResponseWriter, r *http.Request) {
	listing, err := library.Browse(r.URL.Query().Get("dir"))
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	reply(w, listing)
}
