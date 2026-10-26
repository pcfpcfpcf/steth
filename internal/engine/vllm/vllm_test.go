package vllm

import (
	"context"
	"testing"
	"time"

	"github.com/pcfpcfpcf/steth/internal/engine/enginetest"
)

var start = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

func TestTwoReadsASecondApart(t *testing.T) {
	server := enginetest.Serve(t, map[string]string{
		"/v1/models": "testdata/models.json",
		"/metrics":   "testdata/metrics.1.txt",
	})
	source := New(server.URL, server.Client())
	clock := start
	source.now = func() time.Time { return clock }
	first, err := source.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Models) != 1 || first.Models[0].DecodeTPS != nil {
		t.Fatalf("first read %+v; expected the base model alone, without a speed", first.Models)
	}
	server.Answer("/metrics", "testdata/metrics.2.txt")
	clock = start.Add(time.Second)
	second, err := source.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	enginetest.Match(t, second, "testdata/serving.want.json")
}

func TestSleeping(t *testing.T) {
	server := enginetest.Serve(t, map[string]string{
		"/v1/models": "testdata/models.json",
		"/metrics":   "testdata/sleeping.txt",
	})
	reading, err := New(server.URL, server.Client()).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := reading.Models[0].State; got != "sleeping" {
		t.Errorf("state %s, expected sleeping", got)
	}
}

func TestReadFailsWithoutMetrics(t *testing.T) {
	server := enginetest.Serve(t, map[string]string{"/v1/models": "testdata/models.json"})
	if _, err := New(server.URL, server.Client()).Read(context.Background()); err == nil {
		t.Error("expected an error without /metrics")
	}
}
