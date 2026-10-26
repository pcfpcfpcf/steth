package vllm

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/pcfpcfpcf/steth/internal/engine/enginetest"
	"github.com/pcfpcfpcf/steth/internal/engine/local"
)

var start = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

func TestTwoReadsASecondApart(t *testing.T) {
	server := enginetest.Serve(t, map[string]string{
		"/v1/models": "testdata/models.json",
		"/metrics":   "testdata/metrics.1.txt",
	})
	source := New(server.URL, server.Client())
	source.local.Processes = enginetest.Processes{local.Port(server.URL): {
		PID:  5150,
		Args: "/usr/bin/python3 /usr/local/bin/vllm serve Qwen/Qwen3-8B --gpu-memory-utilization 0.9",
		Held: map[string]uint64{"vram": 21 << 30, "dram": 3 << 30},
	}}
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
	source := New(server.URL, server.Client())
	source.local.Processes = enginetest.Processes{}
	reading, err := source.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := reading.Models[0].State; got != "sleeping" {
		t.Errorf("state %s, expected sleeping", got)
	}
}

func TestReadFailsWithoutMetrics(t *testing.T) {
	server := enginetest.Serve(t, map[string]string{"/v1/models": "testdata/models.json"})
	source := New(server.URL, server.Client())
	source.local.Processes = enginetest.Processes{}
	if _, err := source.Read(context.Background()); err == nil {
		t.Error("expected an error without /metrics")
	}
}

func TestRuns(t *testing.T) {
	for cmdline, want := range map[string]bool{
		"/usr/bin/python3 /usr/local/bin/vllm serve Qwen/Qwen3-8B":           true,
		"vllm serve Qwen/Qwen3-8B --port 8001":                               true,
		"python -m vllm.entrypoints.openai.api_server --model Qwen/Qwen3-8B": true,
		"vllm bench serve": false,
		"python -c from vllm.v1.engine.core import run_engine_core": false,
	} {
		if got := Runs(strings.Fields(cmdline)); got != want {
			t.Errorf("%q: %v, expected %v", cmdline, got, want)
		}
	}
}

func TestProbe(t *testing.T) {
	for file, want := range map[string]bool{
		"testdata/models.json":                    true,
		"../llamacpp/testdata/router.models.json": false,
	} {
		server := enginetest.Serve(t, map[string]string{"/v1/models": file})
		if got := Probe(context.Background(), server.Client(), server.URL); got != want {
			t.Errorf("%s: %v, expected %v", file, got, want)
		}
	}
}
