package library

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPlacesAddRemoveAndOutlastSteth(t *testing.T) {
	root := tree(t, map[string]int{"fixed/a.gguf": 1, "more/b.gguf": 2, "c.gguf": 3, "notes.txt": 4})
	at := func(name string) string { return filepath.Join(root, filepath.FromSlash(name)) }
	file := filepath.Join(t.TempDir(), "steth", "places.json")
	places, err := Open([]string{at("fixed")}, file)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{at("more"), at("c.gguf")} {
		if err := places.Add(path); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct{ path, says string }{
		{at("fixed"), "is a place already"},
		{at("more"), "is a place already"},
		{at("notes.txt"), "is neither a directory nor a .gguf file"},
		{at("gone"), "no such file"},
		{"more", "is not an absolute path"},
	} {
		if err := places.Add(c.path); err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("Add(%q): got %v; expected an error saying %q", c.path, err, c.says)
		}
	}
	if err := places.Remove(at("fixed")); err == nil || !strings.Contains(err.Error(), "comes from the config file") {
		t.Errorf("removing a fixed place: %v", err)
	}
	want := []Place{{at("fixed"), true}, {at("more"), false}, {at("c.gguf"), false}}
	if got := places.All(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nexpected %+v", got, want)
	}

	again, err := Open([]string{at("fixed")}, file)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.All(); !reflect.DeepEqual(got, want) {
		t.Errorf("after a restart: got %+v\nexpected %+v", got, want)
	}
	if err := again.Remove(at("more")); err != nil {
		t.Fatal(err)
	}
	if err := again.Remove(at("more")); err == nil || !strings.Contains(err.Error(), "is not a place") {
		t.Errorf("removing twice: %v", err)
	}
	third, _ := Open(nil, file)
	if got := third.Paths(); !reflect.DeepEqual(got, []string{at("c.gguf")}) {
		t.Errorf("after a remove and a restart: %q", got)
	}
}

func TestPlacesFileThatIsNotSteths(t *testing.T) {
	file := filepath.Join(t.TempDir(), "places.json")
	os.WriteFile(file, []byte("not json"), 0o600)
	if _, err := Open(nil, file); err == nil || !strings.Contains(err.Error(), "written by steth") {
		t.Errorf("got %v; expected a refusal naming the file", err)
	}
	// Without a file, places live as long as steth.
	places, err := Open(nil, "")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := places.Add(dir); err != nil || !reflect.DeepEqual(places.Paths(), []string{dir}) {
		t.Errorf("in memory: %v, %q", err, places.Paths())
	}
}

func TestBrowseListsDirectoriesAndModelFiles(t *testing.T) {
	root := tree(t, map[string]int{"b/x": 1, "a/x": 1, ".hidden/x": 1, "m.gguf": 5, "mmproj.gguf": 1, "notes.txt": 1})
	got, err := Browse(root)
	if err != nil {
		t.Fatal(err)
	}
	want := Listing{Dir: root, Parent: filepath.Dir(root), Dirs: []string{"a", "b"},
		Files: []File{{filepath.Join(root, "m.gguf"), 5}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nexpected %+v", got, want)
	}
	for _, dir := range []string{"relative", filepath.Join(root, "gone"), filepath.Join(root, "notes.txt")} {
		if _, err := Browse(dir); err == nil {
			t.Errorf("Browse(%q): expected an error", dir)
		}
	}
	home, err := Browse("")
	if err != nil || home.Dir == "" {
		t.Errorf("Browse of home: %+v, %v", home, err)
	}
	top, _ := filepath.Abs(string(filepath.Separator))
	if l, err := Browse(top); err != nil || l.Parent != "" {
		t.Errorf("the top has no parent: %+v, %v", l.Parent, err)
	}
}
