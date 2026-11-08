package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/pcfpcfpcf/steth/internal/engine/fetch"
	"github.com/pcfpcfpcf/steth/internal/state"
)

// page stands in for the browser: it keeps what Send writes and counts
// events and flushes. Its clock is one second per event written, so a
// time names which event it was read at.
type page struct {
	bytes.Buffer
	events  int
	flushes int
}

func (p *page) Write(b []byte) (int, error) {
	p.events++
	return p.Buffer.Write(b)
}

func (p *page) flush() { p.flushes++ }

func (p *page) now() time.Time {
	return time.Unix(0, 0).Add(time.Duration(p.events) * time.Second)
}

// replay serves a recorded answer to POST /v1/chat/completions and keeps
// the request body it was sent.
func replay(t *testing.T, status int, file string, got *[]byte) *httptest.Server {
	t.Helper()
	answer, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		*got, _ = io.ReadAll(r.Body)
		w.WriteHeader(status)
		w.Write(answer)
	}))
	t.Cleanup(server.Close)
	return server
}

func ptr[T any](v T) *T { return &v }

func TestRepliesFromRecordedStreams(t *testing.T) {
	m, d := state.Measured, state.Derived
	for _, tc := range []struct {
		file    string
		trailer string // bytes after the last event, which are not one
		events  int
		want    Reply
	}{{
		file: "testdata/geode.sse", trailer: "\x00", events: 11,
		want: Reply{
			FirstTextSeconds: ptr(2.0), DecodeTPS: ptr(9.4), PrefillTPS: ptr(29.5),
			Fidelity: map[string]string{"first_text_s": m, "decode_tps": m, "prefill_tps": m},
		},
	}, {
		// The first text is reasoning_content; usage and timings share the
		// last chunk.
		file: "testdata/llamacpp.sse", events: 24,
		want: Reply{
			FirstTextSeconds: ptr(2.0), DecodeTPS: ptr(18.585253344880968), PrefillTPS: ptr(11.797904839574302),
			PromptTokens: ptr[int64](16), CompletionTokens: ptr[int64](20),
			Fidelity: map[string]string{"first_text_s": m, "decode_tps": m, "prefill_tps": m, "prompt_tokens": m, "completion_tokens": m},
		},
	}, {
		// No timings: 8 tokens, the 7 after the first in the 9 s from
		// event 1 to event 10.
		file: "testdata/ollama.sse", events: 10,
		want: Reply{
			FirstTextSeconds: ptr(1.0), DecodeTPS: ptr(7.0 / 9),
			PromptTokens: ptr[int64](35), CompletionTokens: ptr[int64](8),
			Fidelity: map[string]string{"first_text_s": m, "decode_tps": d, "prompt_tokens": m, "completion_tokens": m},
		},
	}} {
		t.Run(tc.file, func(t *testing.T) {
			var sent []byte
			server := replay(t, http.StatusOK, tc.file, &sent)
			var p page
			target := Target{Engine: "any", Endpoint: server.URL, Model: "m"}
			req := Request{Messages: []Message{{Role: "user", Content: "hi"}}, MaxTokens: ptr(20)}
			got, err := send(context.Background(), server.Client(), target, req, &p, p.flush, p.now)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				gotJSON, _ := json.Marshal(got)
				wantJSON, _ := json.Marshal(tc.want)
				t.Errorf("reply\n got %s\nwant %s", gotJSON, wantJSON)
			}
			recorded, _ := os.ReadFile(tc.file)
			if want := bytes.TrimSuffix(recorded, []byte(tc.trailer)); !bytes.Equal(p.Bytes(), want) {
				t.Errorf("page got %d bytes, expected the recording's %d verbatim", p.Len(), len(want))
			}
			if p.events != tc.events || p.flushes != tc.events {
				t.Errorf("%d events written and %d flushes, expected %d of each", p.events, p.flushes, tc.events)
			}
			var wire map[string]any
			if err := json.Unmarshal(sent, &wire); err != nil {
				t.Fatal(err)
			}
			want := map[string]any{
				"model":          "m",
				"messages":       []any{map[string]any{"role": "user", "content": "hi"}},
				"max_tokens":     20.0,
				"stream":         true,
				"stream_options": map[string]any{"include_usage": true},
			}
			if !reflect.DeepEqual(wire, want) {
				t.Errorf("engine was sent %s", sent)
			}
		})
	}
}

func TestRefusalCarriesEngineMessage(t *testing.T) {
	for _, tc := range []struct {
		file    string
		status  int
		message string
	}{
		{"testdata/geode-refused.json", http.StatusBadRequest, "unknown model 'nope'; available: Qwen3.5-0.8B"},
		{"testdata/ollama-refused.json", http.StatusInternalServerError, "llama runner process has terminated: %!w(<nil>)"},
	} {
		var sent []byte
		server := replay(t, tc.status, tc.file, &sent)
		var p page
		_, err := Send(context.Background(), server.Client(), Target{Endpoint: server.URL, Model: "nope"}, Request{}, &p, p.flush)
		var refused *fetch.StatusError
		if !errors.As(err, &refused) || refused.Status != tc.status || refused.Message != tc.message {
			t.Errorf("%s: got %v, expected status %d with %q", tc.file, err, tc.status, tc.message)
		}
		if p.Len() != 0 || p.flushes != 0 {
			t.Errorf("%s: page got %q and %d flushes, expected nothing", tc.file, p.String(), p.flushes)
		}
	}
}

func TestCancelClosesEngineRequest(t *testing.T) {
	ended := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(ended)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var p page
	done := make(chan error, 1)
	go func() {
		_, err := Send(ctx, server.Client(), Target{Endpoint: server.URL, Model: "m"}, Request{}, &p, cancel)
		done <- err
	}()
	select {
	case <-ended:
	case <-time.After(5 * time.Second):
		t.Fatal("the engine's request was still open 5 s after the page cancelled")
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Errorf("got %v, expected context.Canceled", err)
	}
}
