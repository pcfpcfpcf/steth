package library

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// browseMost bounds the entries one listing returns.
const browseMost = 2000

// Listing is one directory, for picking a place: its subdirectories and the
// model files directly in it.
type Listing struct {
	Dir string `json:"dir"`
	// Parent is the directory above, "" at the top of a volume.
	Parent string   `json:"parent"`
	Dirs   []string `json:"dirs"` // names, sorted
	Files  []File   `json:"files"`
	// Cut says the directory has more entries than are listed.
	Cut bool `json:"cut"`
}

// Browse lists dir, an absolute directory; "" is the home directory.
// Hidden entries are left out; a link to a directory lists as one.
func Browse(dir string) (Listing, error) {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Listing{}, err
		}
		dir = home
	}
	if err := Check(dir); err != nil {
		return Listing{}, err
	}
	dir = filepath.Clean(dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Listing{}, err
	}
	l := Listing{Dir: dir, Dirs: []string{}, Files: []File{}}
	if parent := filepath.Dir(dir); parent != dir {
		l.Parent = parent
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if len(l.Dirs)+len(l.Files) >= browseMost {
			l.Cut = true
			break
		}
		path := filepath.Join(dir, e.Name())
		if f, ok := modelFile(path); ok {
			l.Files = append(l.Files, f)
			continue
		}
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			l.Dirs = append(l.Dirs, e.Name())
		}
	}
	slices.Sort(l.Dirs)
	return l, nil
}
