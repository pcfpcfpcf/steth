package launch

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// tree writes files of the given sizes under a new directory and returns it.
func tree(t *testing.T, sizes map[string]int) string {
	t.Helper()
	root := t.TempDir()
	for name, size := range sizes {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestScanListsModelFiles(t *testing.T) {
	root := tree(t, map[string]int{
		"b.gguf":                     2,
		"A.GGUF":                     1,
		"org/repo/c.gguf":            3,
		"1/2/3/4/at-depth.gguf":      4,
		"1/2/3/4/5/too-deep.gguf":    5,
		".cache/hidden.gguf":         6,
		".hidden.gguf":               6,
		"mmproj-F16.gguf":            7,
		"notes.txt":                  8,
		"big-00001-of-00002.gguf":    10,
		"big-00002-of-00002.gguf":    20,
		"other-00002-of-00002.gguf":  30, // no first part: not listed
		"big-00001-of-00003.gguf/no": 1,  // a directory named like a model
	})
	alone := tree(t, map[string]int{"alone.gguf": 9})
	file := filepath.Join(alone, "alone.gguf")
	at := func(name string) string { return filepath.Join(root, filepath.FromSlash(name)) }

	got := Scan([]string{root, file, filepath.Join(root, "gone"), root})
	want := []File{
		{at("1/2/3/4/at-depth.gguf"), 4},
		{at("A.GGUF"), 1},
		{at("b.gguf"), 2},
		{at("big-00001-of-00002.gguf"), 30},
		{at("org/repo/c.gguf"), 3},
		{file, 9},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nexpected %+v", got, want)
	}
}

// A Hugging Face cache links each snapshot's file to a blob.
func TestScanFollowsLinkedFiles(t *testing.T) {
	blobs := tree(t, map[string]int{"blobs/0123abcd": 5})
	link := filepath.Join(blobs, "snapshots", "main", "m.gguf")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(blobs, "blobs", "0123abcd"), link); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	if got := Scan([]string{blobs}); !reflect.DeepEqual(got, []File{{link, 5}}) {
		t.Errorf("got %+v; expected the link with its blob's size", got)
	}
}

func TestLoadChecksModels(t *testing.T) {
	root := tree(t, map[string]int{"m.gguf": 1, "notes.txt": 1})
	quote := func(s string) string { return strings.ReplaceAll(s, `\`, `\\`) }
	takes := `"launch": [{"name": "a", "command": ["x", "{model}"]}]`
	for _, c := range []struct{ models, says string }{
		{`["` + quote(filepath.Join(root, "gone")) + `"]`, "models: stat"},
		{`["` + quote(filepath.Join(root, "notes.txt")) + `"]`, "is neither a directory nor a .gguf file"},
	} {
		_, err := Load(writeConfig(t, `{`+takes+`, "models": `+c.models+`}`))
		if err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("models %s: got %v; expected an error saying %q", c.models, err, c.says)
		}
	}
	got, err := Load(writeConfig(t, `{`+takes+`, "models": ["`+quote(root)+`", "`+quote(filepath.Join(root, "m.gguf"))+`"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Models, []string{root, filepath.Join(root, "m.gguf")}) {
		t.Errorf("models %q", got.Models)
	}
}

func TestProfileTakesTheModelFiles(t *testing.T) {
	root := tree(t, map[string]int{"m1.gguf": 1})
	echo := helper(t, "files", "echo", "{model}")
	own := helper(t, "own", "echo", "{model}")
	own.Models = []string{"listed"}
	l := New(Config{Launch: []Profile{echo, own}, Models: []string{root}})
	l.grace = shortGrace
	t.Cleanup(l.StopAll)

	m1, m2 := filepath.Join(root, "m1.gguf"), filepath.Join(root, "m2.gguf")
	if err := l.Start("files", m2); err == nil || !strings.Contains(err.Error(), `has no model "`) {
		t.Errorf("a file not there yet: got %v; expected a refusal", err)
	}
	if err := os.WriteFile(m2, []byte("12"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := status(t, l, "files")
	if want := []File{{m1, 1}, {m2, 2}}; !reflect.DeepEqual(s.Files, want) || !reflect.DeepEqual(s.Models, []string{m1, m2}) {
		t.Errorf("status %+v; expected the files found now, %+v", s, want)
	}
	if s := status(t, l, "own"); s.Files != nil || !reflect.DeepEqual(s.Models, []string{"listed"}) {
		t.Errorf("a profile with its own models: %+v", s)
	}
	if err := l.Start("own", m1); err == nil {
		t.Error("a profile with its own models took a model file")
	}
	start(t, l, "files", m2)
}

func TestScanStopsAtItsBound(t *testing.T) {
	sizes := map[string]int{}
	for i := range scanMost + 5 {
		sizes[fmt.Sprintf("m%04d.gguf", i)] = 0
	}
	if got := len(Scan([]string{tree(t, sizes)})); got != scanMost {
		t.Errorf("listed %d of %d files; expected %d", got, len(sizes), scanMost)
	}
}

// A profile that takes model files says it found none, apart from one with
// its own models.
func TestNoModelFilesIsAnEmptyList(t *testing.T) {
	l := New(Config{Launch: []Profile{helper(t, "files", "echo", "{model}")}, Models: []string{t.TempDir()}})
	if s := status(t, l, "files"); s.Files == nil || len(s.Files) != 0 || len(s.Models) != 0 {
		t.Errorf("status %+v; expected an empty list of files", s)
	}
	if err := l.Start("files", ""); err == nil || !strings.Contains(err.Error(), "no model files are under") {
		t.Errorf("got %v; expected a refusal saying no files were found", err)
	}
}

func TestRefusalNamesFewChoices(t *testing.T) {
	p := helper(t, "many", "echo", "{model}")
	p.Models = []string{"a", "b", "c", "d", "e", "f", "g"}
	err := New(Config{Launch: []Profile{p}}).Start("many", "z")
	if want := `choose one of "a", "b", "c", "d", "e" and 2 more`; err == nil || !strings.HasSuffix(err.Error(), want) {
		t.Errorf("got %v; expected it to end %q", err, want)
	}
}
