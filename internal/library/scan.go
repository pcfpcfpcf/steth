package library

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const (
	modelExt = ".gguf"
	// scanDepth is how many directories deep a scan looks under a models
	// directory: models/org/repo/file.gguf is two.
	scanDepth = 4
	// scanMost bounds the files one scan lists, so a models entry naming
	// a whole disk stays a short walk.
	scanMost = 1000
)

// shard matches a split model's part: name-00002-of-00003.gguf.
var shard = regexp.MustCompile(`(?i)^(.*)-(\d{5})-of-(\d{5})\.gguf$`)

// File is a model file found under the config's models.
type File struct {
	Path string `json:"path"`
	// Bytes is the file's size on disk; for a split model, all its parts
	// found beside the first.
	Bytes int64 `json:"bytes"`
}

// Scan lists the .gguf model files under roots, in roots' order, each
// root's files sorted by path. A root is a .gguf file or a directory looked
// through scanDepth deep, skipping hidden entries. A split model is listed
// once, by its first part; a vision projector (mmproj*.gguf) is not a model
// and is left out. A root that is gone is skipped: the config named it when
// steth started, and a disk can be unmounted since.
func Scan(roots []string) []File {
	var files []File
	seen := map[string]bool{}
	add := func(f File) {
		if len(files) < scanMost && !seen[f.Path] {
			seen[f.Path] = true
			files = append(files, f)
		}
	}
	for _, root := range roots {
		info, err := os.Stat(root)
		switch {
		case err != nil:
			continue
		case !info.IsDir():
			add(File{Path: filepath.Clean(root), Bytes: info.Size()})
			continue
		}
		var found []File
		filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // an unreadable entry; the rest still list
			}
			if path != root && strings.HasPrefix(d.Name(), ".") {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				if depth(root, path) > scanDepth {
					return fs.SkipDir
				}
				return nil
			}
			if f, ok := modelFile(path); ok {
				found = append(found, f)
			}
			return nil
		})
		slices.SortFunc(found, func(a, b File) int { return strings.Compare(a.Path, b.Path) })
		for _, f := range found {
			add(f)
		}
	}
	return files
}

// modelFile is the model at path, or false when path is not a model's
// file or first part. A symbolic link counts as what it points to, as in a
// Hugging Face cache, whose snapshots link to blobs.
func modelFile(path string) (File, bool) {
	name := filepath.Base(path)
	if !strings.EqualFold(filepath.Ext(name), modelExt) || strings.HasPrefix(strings.ToLower(name), "mmproj") {
		return File{}, false
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return File{}, false
	}
	part := shard.FindStringSubmatch(name)
	if part == nil {
		return File{Path: path, Bytes: info.Size()}, true
	}
	if part[2] != "00001" {
		return File{}, false
	}
	bytes := info.Size()
	dir := filepath.Dir(path)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		other := shard.FindStringSubmatch(e.Name())
		if other == nil || other[1] != part[1] || other[3] != part[3] || other[2] == "00001" {
			continue
		}
		if info, err := os.Stat(filepath.Join(dir, e.Name())); err == nil && info.Mode().IsRegular() {
			bytes += info.Size()
		}
	}
	return File{Path: path, Bytes: bytes}, true
}

// depth is how many directories below root dir is.
func depth(root, dir string) int {
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." {
		return 0
	}
	return strings.Count(rel, string(filepath.Separator)) + 1
}

// paths are the files' paths, in order.
func paths(files []File) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Path
	}
	return out
}
