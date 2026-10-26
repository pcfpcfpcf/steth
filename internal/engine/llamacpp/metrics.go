package llamacpp

import (
	"context"
	"fmt"
	"strings"

	"github.com/pcfpcfpcf/steth/internal/engine/fetch"
	"github.com/pcfpcfpcf/steth/internal/engine/prom"
	"github.com/pcfpcfpcf/steth/internal/state"
)

const metricPrefix = "llamacpp:"

// counters are the running totals a speed is derived from. Counters, not
// the server's own throughput gauges: every scrape resets those, so they
// would share a window with any other scraper.
type counters struct {
	predicted, predictedSeconds, prompt, promptSeconds float64
}

// readMetrics fills `m` from GET /metrics: requests running and deferred,
// and each speed since the last read, over the time spent on it.
func (s *Source) readMetrics(ctx context.Context, model string, m *state.Model) error {
	body, err := fetch.Body(ctx, s.client, s.address("/metrics", model))
	if err != nil {
		return err
	}
	values, err := parseMetrics(body)
	if err != nil {
		return fmt.Errorf("GET %s: %w", s.address("/metrics", model), err)
	}
	if v, ok := values["requests_processing"]; ok {
		running := int64(v)
		m.Running = &running
		m.Fidelity["running"] = state.Measured
	}
	if v, ok := values["requests_deferred"]; ok {
		queued := int64(v)
		m.Queued = &queued
		m.Fidelity["queued"] = state.Measured
	}
	now := counters{
		predicted:        values["tokens_predicted_total"],
		predictedSeconds: values["tokens_predicted_seconds_total"],
		prompt:           values["prompt_tokens_total"],
		promptSeconds:    values["prompt_seconds_total"],
	}
	s.next[model] = now
	before, ok := s.last[model]
	if !ok {
		return nil
	}
	if m.DecodeTPS = rate(now.predicted-before.predicted, now.predictedSeconds-before.predictedSeconds); m.DecodeTPS != nil {
		m.Fidelity["decode_tps"] = state.Derived
	}
	if m.PrefillTPS = rate(now.prompt-before.prompt, now.promptSeconds-before.promptSeconds); m.PrefillTPS != nil {
		m.Fidelity["prefill_tps"] = state.Derived
	}
	return nil
}

// rate is tokens over the seconds spent on them; nil when none were spent,
// or when the counters went back, as they do when the server restarts.
func rate(tokens, seconds float64) *float64 {
	if seconds <= 0 || tokens < 0 {
		return nil
	}
	r := tokens / seconds
	return &r
}

// parseMetrics reads the unlabelled llamacpp: samples of a metrics body,
// by name without the prefix.
func parseMetrics(body []byte) (map[string]float64, error) {
	samples, err := prom.Parse(body)
	if err != nil {
		return nil, err
	}
	values := map[string]float64{}
	for _, s := range samples {
		if strings.HasPrefix(s.Name, metricPrefix) && len(s.Labels) == 0 {
			values[strings.TrimPrefix(s.Name, metricPrefix)] = s.Value
		}
	}
	return values, nil
}
