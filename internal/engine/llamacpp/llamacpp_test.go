package llamacpp

import (
	"context"
	"testing"

	"github.com/pcfpcfpcf/steth/internal/engine/enginetest"
)

const child = "?autoload=false&model=qwen3.5-0.8b"

func TestReadMatchesSchema(t *testing.T) {
	cases := map[string]map[string]string{
		"single": {
			"/props": "testdata/single.props.json",
			"/slots": "testdata/single.slots.json",
		},
		// No /slots: reading it would wake the server.
		"sleeping": {
			"/props": "testdata/sleeping.props.json",
		},
		"router": {
			"/props":         "testdata/router.props.json",
			"/v1/models":     "testdata/router.models.json",
			"/props" + child: "testdata/router.child.props.json",
			"/slots" + child: "testdata/router.child.slots.json",
		},
	}
	for name, fixtures := range cases {
		t.Run(name, func(t *testing.T) {
			server := enginetest.Serve(t, fixtures)
			reading, err := New(server.URL, server.Client()).Read(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			enginetest.Match(t, reading, "testdata/"+name+".want.json")
		})
	}
}

func TestReadFailsOnBadStatus(t *testing.T) {
	server := enginetest.Serve(t, map[string]string{})
	if _, err := New(server.URL, server.Client()).Read(context.Background()); err == nil {
		t.Fatal("expected an error for a 404")
	}
}
