package library

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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

func TestScanStopsAtItsBound(t *testing.T) {
	sizes := map[string]int{}
	for i := range scanMost + 5 {
		sizes[fmt.Sprintf("m%04d.gguf", i)] = 0
	}
	if got := len(Scan([]string{tree(t, sizes)})); got != scanMost {
		t.Errorf("listed %d of %d files; expected %d", got, len(sizes), scanMost)
	}
}
