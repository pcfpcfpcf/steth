package geode

import (
	"context"
	"strings"
	"testing"

	"github.com/pcfpcfpcf/steth/internal/engine/enginetest"
)

func TestReadMatchesSchema(t *testing.T) {
	server := enginetest.Serve(t, map[string]string{"/v1/pool": "testdata/idle.json"})
	reading, err := New(server.URL, server.Client()).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	enginetest.Match(t, reading, "testdata/idle.want.json")
}

func TestReadFailsOnBadStatus(t *testing.T) {
	server := enginetest.Serve(t, map[string]string{})
	if _, err := New(server.URL, server.Client()).Read(context.Background()); err == nil {
		t.Fatal("expected an error for a 404")
	}
}

func TestRuns(t *testing.T) {
	for cmdline, want := range map[string]bool{
		"bin/geode serve a.gguf b.gguf --port 18080": true,
		"bin/geode manifest a.gguf":                  false,
	} {
		if got := Runs(strings.Fields(cmdline)); got != want {
			t.Errorf("%q: %v, expected %v", cmdline, got, want)
		}
	}
}

func TestProbe(t *testing.T) {
	for file, want := range map[string]bool{
		"testdata/idle.json":              true,
		"../ollama/testdata/version.json": false,
	} {
		server := enginetest.Serve(t, map[string]string{"/v1/pool": file})
		if got := Probe(context.Background(), server.Client(), server.URL); got != want {
			t.Errorf("%s: %v, expected %v", file, got, want)
		}
	}
}

// Speeds come from the sums between two reads, recorded from a running
// server 3 s apart while it answered one request: 14 prompt tokens in
// 0.49 s, 22 decoded in 2.38 s. The first read has no speed.
func TestSpeedsFromSums(t *testing.T) {
	server := enginetest.Serve(t, map[string]string{"/v1/pool": "testdata/serving.1.json"})
	source := New(server.URL, server.Client())
	first, err := source.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if m := first.Models[0]; m.DecodeTPS != nil || m.PrefillTPS != nil {
		t.Fatalf("first read has speeds %v, %v; expected none", *m.DecodeTPS, *m.PrefillTPS)
	}
	server.Answer("/v1/pool", "testdata/serving.2.json")
	second, err := source.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	enginetest.Match(t, second, "testdata/serving.want.json")
}
