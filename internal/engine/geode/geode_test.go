package geode

import (
	"context"
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
