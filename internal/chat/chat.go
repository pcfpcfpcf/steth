// Package chat sends one prompt to a model as a streamed OpenAI chat
// completion, passes the engine's events through as they arrive, and
// reports that reply's numbers.
package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pcfpcfpcf/steth/internal/engine/fetch"
	"github.com/pcfpcfpcf/steth/internal/state"
)

// The longest line of an event an engine may send.
const maxEventLine = 1 << 20

// Target is the model to send to. Endpoint is the engine's base URL as
// steth reads it, e.g. http://127.0.0.1:11434.
type Target struct{ Engine, Endpoint, Model string }

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	Messages    []Message `json:"messages"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
	Temperature *float64  `json:"temperature,omitempty"`
}

// Reply is one reply's numbers; nil is "not said". Fidelity maps each
// non-nil field's json name to state.Measured or state.Derived.
type Reply struct {
	FirstTextSeconds *float64          `json:"first_text_s"`
	DecodeTPS        *float64          `json:"decode_tps"`
	PrefillTPS       *float64          `json:"prefill_tps"`
	PromptTokens     *int64            `json:"prompt_tokens"`
	CompletionTokens *int64            `json:"completion_tokens"`
	Fidelity         map[string]string `json:"fidelity"`
}

// Send posts req to target as a streamed OpenAI chat completion, copies
// each event the engine sends to w as it arrives, calling flush after each,
// and returns the reply's numbers once the stream ends. An answer other
// than 200 is a *fetch.StatusError with the engine's message and writes
// nothing to w. ctx ending closes the engine's request. client must not
// have a Timeout shorter than a reply.
func Send(ctx context.Context, client *http.Client, target Target, req Request, w io.Writer, flush func()) (Reply, error) {
	return send(ctx, client, target, req, w, flush, time.Now)
}

type wireRequest struct {
	Model string `json:"model"`
	Request
	Stream        bool `json:"stream"`
	StreamOptions struct {
		IncludeUsage bool `json:"include_usage"`
	} `json:"stream_options"`
}

func send(ctx context.Context, client *http.Client, target Target, req Request, w io.Writer, flush func(), now func() time.Time) (Reply, error) {
	wire := wireRequest{Model: target.Model, Request: req, Stream: true}
	wire.StreamOptions.IncludeUsage = true
	body, err := json.Marshal(wire)
	if err != nil {
		return Reply{}, err
	}
	url := target.Endpoint + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Reply{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	sent := now()
	resp, err := client.Do(httpReq)
	if err != nil {
		return Reply{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Reply{}, fetch.Refusal(resp)
	}

	var t tally
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(nil, maxEventLine)
	var event bytes.Buffer
	var data []string
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			event.WriteString(line)
			event.WriteByte('\n')
			if d, ok := strings.CutPrefix(line, "data:"); ok {
				data = append(data, strings.TrimPrefix(d, " "))
			}
			continue
		}
		if event.Len() == 0 {
			continue
		}
		event.WriteByte('\n')
		if _, err := w.Write(event.Bytes()); err != nil {
			return Reply{}, err
		}
		flush()
		t.add(strings.Join(data, "\n"), now)
		event.Reset()
		data = data[:0]
	}
	// An event the stream ends without a blank line after is dropped, as
	// SSE says; geode ends its stream with a stray NUL byte.
	if err := scanner.Err(); err != nil {
		return Reply{}, fmt.Errorf("reading the reply from %s: %w", url, err)
	}
	return t.reply(sent, now()), nil
}

// wireTimings holds both engines' names; they do not collide, so which
// engine sent them never needs asking.
type wireTimings struct {
	DecodeTokS         *float64 `json:"decode_tok_s"`         // geode
	PrefillTokS        *float64 `json:"prefill_tok_s"`        // geode
	PredictedPerSecond *float64 `json:"predicted_per_second"` // llama.cpp
	PromptPerSecond    *float64 `json:"prompt_per_second"`    // llama.cpp
}

type wireChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     *int64 `json:"prompt_tokens"`
		CompletionTokens *int64 `json:"completion_tokens"`
	} `json:"usage"`
	Timings *wireTimings `json:"timings"`
}

// tally is what the stream has said so far.
type tally struct {
	firstText        time.Time
	timings          wireTimings
	promptTokens     *int64
	completionTokens *int64
}

// add reads one event's data. Data that is not a chunk ([DONE], or
// anything else) is passed through without counting.
func (t *tally) add(data string, now func() time.Time) {
	var chunk wireChunk
	if json.Unmarshal([]byte(data), &chunk) != nil {
		return
	}
	if t.firstText.IsZero() {
		for _, c := range chunk.Choices {
			if c.Delta.Content != "" || c.Delta.ReasoningContent != "" {
				t.firstText = now()
				break
			}
		}
	}
	if chunk.Timings != nil {
		t.timings = *chunk.Timings
	}
	if chunk.Usage != nil {
		t.promptTokens = chunk.Usage.PromptTokens
		t.completionTokens = chunk.Usage.CompletionTokens
	}
}

func (t *tally) reply(sent, ended time.Time) Reply {
	r := Reply{Fidelity: map[string]string{}}
	set := func(field string, ok bool) {
		if ok {
			r.Fidelity[field] = state.Measured
		}
	}
	if !t.firstText.IsZero() {
		r.FirstTextSeconds = seconds(t.firstText.Sub(sent))
		set("first_text_s", true)
	}
	r.PromptTokens, r.CompletionTokens = t.promptTokens, t.completionTokens
	set("prompt_tokens", r.PromptTokens != nil)
	set("completion_tokens", r.CompletionTokens != nil)
	r.DecodeTPS = first(t.timings.DecodeTokS, t.timings.PredictedPerSecond)
	r.PrefillTPS = first(t.timings.PrefillTokS, t.timings.PromptPerSecond)
	set("decode_tps", r.DecodeTPS != nil)
	set("prefill_tps", r.PrefillTPS != nil)
	if r.DecodeTPS == nil {
		r.DecodeTPS = t.derivedDecode(ended)
		if r.DecodeTPS != nil {
			r.Fidelity["decode_tps"] = state.Derived
		}
	}
	return r
}

// derivedDecode counts the tokens after the first one, since the first
// text marks when decoding started, over the seconds to the stream's end.
func (t *tally) derivedDecode(ended time.Time) *float64 {
	if t.firstText.IsZero() || t.completionTokens == nil || *t.completionTokens < 2 {
		return nil
	}
	elapsed := ended.Sub(t.firstText).Seconds()
	if elapsed <= 0 {
		return nil
	}
	tps := float64(*t.completionTokens-1) / elapsed
	return &tps
}

func seconds(d time.Duration) *float64 {
	s := d.Seconds()
	return &s
}

func first(values ...*float64) *float64 {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}
