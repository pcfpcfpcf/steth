package vllm

import (
	"context"
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/pcfpcfpcf/steth/internal/engine/control"
	"github.com/pcfpcfpcf/steth/internal/engine/enginetest"
)

var _ control.Controls = (*Source)(nil)

// engine answers each path with its file, and keeps every POST it was sent.
func engine(t *testing.T, answers map[string]string) (*Source, func() []string) {
	server := enginetest.Serve(t, answers)
	return New(server.URL, server.Client()), server.Posted
}

// A server in dev mode with sleep mode on, written from vLLM's source.
var devMode = map[string]string{
	"/v1/models":   "testdata/models.json",
	"/is_sleeping": "testdata/is_sleeping.json",
	"/sleep":       "testdata/sleep.json",
	"/wake_up":     "testdata/wake_up.json",
}

func TestSleepsAndWakesItsOneModel(t *testing.T) {
	ctx := context.Background()
	source, sent := engine(t, devMode)
	got, err := source.Available(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Qwen/Qwen3-8B"}; !slices.Equal(got, want) {
		t.Errorf("available %q, expected %q without its LoRA adapter", got, want)
	}
	if err := source.Unload(ctx, "Qwen/Qwen3-8B"); err != nil {
		t.Fatal(err)
	}
	if err := source.Load(ctx, "Qwen/Qwen3-8B"); err != nil {
		t.Fatal(err)
	}
	if got, want := sent(), []string{"/sleep?level=1 ", "/wake_up "}; !slices.Equal(got, want) {
		t.Errorf("sent %q, expected %q", got, want)
	}
}

func TestRefusesAModelItDoesNotServe(t *testing.T) {
	source, sent := engine(t, devMode)
	for _, model := range []string{"sql-lora", "other/model"} {
		if err := source.Unload(context.Background(), model); err == nil {
			t.Errorf("%s: put to sleep; expected a refusal", model)
		}
	}
	if got := sent(); len(got) > 0 {
		t.Errorf("sent %q for a model it does not serve", got)
	}
}

func TestNoDevModeHasNoControls(t *testing.T) {
	ctx := context.Background()
	source, _ := engine(t, map[string]string{"/v1/models": "testdata/models.json"})
	if _, err := source.Available(ctx); !errors.Is(err, control.ErrNone) {
		t.Errorf("available: %v; expected ErrNone", err)
	}
	if err := source.Load(ctx, "Qwen/Qwen3-8B"); !errors.Is(err, control.ErrNone) {
		t.Errorf("load: %v; expected ErrNone", err)
	}
	if err := source.Unload(ctx, "Qwen/Qwen3-8B"); !errors.Is(err, control.ErrNone) {
		t.Errorf("unload: %v; expected ErrNone", err)
	}
}

func TestOtherFailuresStayErrors(t *testing.T) {
	answers := maps.Clone(devMode)
	delete(answers, "/v1/models")
	source, _ := engine(t, answers)
	if _, err := source.Available(context.Background()); err == nil || errors.Is(err, control.ErrNone) {
		t.Errorf("available: %v; expected the models list's own failure", err)
	}
}
