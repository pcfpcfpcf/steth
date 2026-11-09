package launch

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pcfpcfpcf/steth/internal/library"
)

// modelDir writes each named file, of len(name) bytes, in a new directory.
func modelDir(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoadChecksModels(t *testing.T) {
	dir := modelDir(t, "m.gguf", "notes.txt")
	quote := func(s string) string { return strings.ReplaceAll(s, `\`, `\\`) }
	takes := `"launch": [{"name": "a", "command": ["x", "{model}"]}]`
	for _, c := range []struct{ models, says string }{
		{`["` + quote(filepath.Join(dir, "gone")) + `"]`, "models: stat"},
		{`["` + quote(filepath.Join(dir, "notes.txt")) + `"]`, "is neither a directory nor a .gguf file"},
		{`["relative"]`, "is not an absolute path"},
	} {
		_, err := Load(writeConfig(t, `{`+takes+`, "models": `+c.models+`}`))
		if err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("models %s: got %v; expected an error saying %q", c.models, err, c.says)
		}
	}
	got, err := Load(writeConfig(t, `{`+takes+`, "models": ["`+quote(dir)+`"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Models, []string{dir}) {
		t.Errorf("models %q", got.Models)
	}
	// A profile may take model files before any place is named: the page
	// adds places.
	if _, err := Load(writeConfig(t, `{`+takes+`}`)); err != nil {
		t.Errorf("a profile taking files with no place yet: %v", err)
	}
}

func TestProfileTakesTheModelFiles(t *testing.T) {
	dir := modelDir(t, "m1.gguf")
	places := []string{dir}
	echo := helper(t, "files", "echo", "{model}")
	own := helper(t, "own", "echo", "{model}")
	own.Models = []string{"listed"}
	l := New([]Profile{echo, own}, func() []string { return places })
	l.grace = shortGrace
	t.Cleanup(l.StopAll)

	m1, m2 := filepath.Join(dir, "m1.gguf"), filepath.Join(dir, "m2.gguf")
	if err := l.Start("files", m2); err == nil || !strings.Contains(err.Error(), `has no model "`) {
		t.Errorf("a file not there yet: got %v; expected a refusal", err)
	}
	other := modelDir(t, "m2.gguf")
	places = append(places, other)
	m2 = filepath.Join(other, "m2.gguf")
	s := status(t, l, "files")
	if want := []library.File{{Path: m1, Bytes: 7}, {Path: m2, Bytes: 7}}; !reflect.DeepEqual(s.Files, want) || !reflect.DeepEqual(s.Models, []string{m1, m2}) {
		t.Errorf("status %+v; expected the files under the places now, %+v", s, want)
	}
	if s := status(t, l, "own"); s.Files != nil || !reflect.DeepEqual(s.Models, []string{"listed"}) {
		t.Errorf("a profile with its own models: %+v", s)
	}
	if err := l.Start("own", m1); err == nil {
		t.Error("a profile with its own models took a model file")
	}
	start(t, l, "files", m2)
}

// A profile that takes model files says it found none, apart from one with
// its own models.
func TestNoModelFilesIsAnEmptyList(t *testing.T) {
	l := New([]Profile{helper(t, "files", "echo", "{model}")}, nil)
	if s := status(t, l, "files"); s.Files == nil || len(s.Files) != 0 || len(s.Models) != 0 {
		t.Errorf("status %+v; expected an empty list of files", s)
	}
	if err := l.Start("files", ""); err == nil || !strings.Contains(err.Error(), "no model files are under the places") {
		t.Errorf("got %v; expected a refusal saying no files were found", err)
	}
}

func TestRefusalNamesFewChoices(t *testing.T) {
	p := helper(t, "many", "echo", "{model}")
	p.Models = []string{"a", "b", "c", "d", "e", "f", "g"}
	err := New([]Profile{p}, nil).Start("many", "z")
	if want := `choose one of "a", "b", "c", "d", "e" and 2 more`; err == nil || !strings.HasSuffix(err.Error(), want) {
		t.Errorf("got %v; expected it to end %q", err, want)
	}
}

func TestPlaceRepeatsWithItsFlag(t *testing.T) {
	p := Profile{Command: []string{"geode", "serve", "{model}", "--models-dir", "{place}", "--x={place}/y", "--port", "1"}}
	for _, c := range []struct {
		places []string
		want   []string
	}{
		{nil, []string{"geode", "serve", "m", "--port", "1"}},
		{[]string{"/a", "/b.gguf"}, []string{"geode", "serve", "m", "--models-dir", "/a", "--models-dir", "/b.gguf",
			"--x=/a/y", "--x=/b.gguf/y", "--port", "1"}},
	} {
		if got := p.argv("m", c.places, 0); !reflect.DeepEqual(got, c.want) {
			t.Errorf("places %q: got %q\nexpected %q", c.places, got, c.want)
		}
	}
}

// A profile whose model is optional starts with none, leaving out each
// argument with {model} and the flag before it; with one, it is as usual.
func TestModelMayBeLeftOut(t *testing.T) {
	p := Profile{Name: "geode", ModelOptional: true,
		Command: []string{"geode", "serve", "{model}", "-m", "{model}", "--models-dir", "{place}", "--port", "{port}"}}
	if got, want := p.argv("", []string{"/a"}, 8081), []string{"geode", "serve", "--models-dir", "/a", "--port", "8081"}; !reflect.DeepEqual(got, want) {
		t.Errorf("no model: got %q\nexpected %q", got, want)
	}
	if got, want := p.argv("/a/m.gguf", nil, 8081), []string{"geode", "serve", "/a/m.gguf", "-m", "/a/m.gguf", "--port", "8081"}; !reflect.DeepEqual(got, want) {
		t.Errorf("a model: got %q\nexpected %q", got, want)
	}
	if err := p.checkModel("", nil); err != nil {
		t.Errorf("no model, optional: %v", err)
	}
	p.ModelOptional = false
	if err := p.checkModel("", nil); err == nil {
		t.Error("no model, required: expected a refusal")
	}

	echo := helper(t, "optional", "echo", "first", "{model}", "last")
	echo.ModelOptional = true
	l := launcher(t, echo)
	if s := status(t, l, "optional"); !s.ModelOptional {
		t.Errorf("status %+v; expected it to say the model is optional", s)
	}
	start(t, l, "optional", "")
	waitForLine(t, l, "optional", "last")
	if log := status(t, l, "optional").Log; !reflect.DeepEqual(log, []string{"first", "last"}) {
		t.Errorf("log %q; expected the command without {model}", log)
	}
}
