package collector

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pcfpcfpcf/steth/internal/state"
)

type fake struct {
	engine  string
	reading state.Reading
	err     error
	delay   time.Duration
}

func (f *fake) Engine() string   { return f.engine }
func (f *fake) Endpoint() string { return "http://" + f.engine }
func (f *fake) Read(ctx context.Context) (state.Reading, error) {
	select {
	case <-time.After(f.delay):
		return f.reading, f.err
	case <-ctx.Done():
		return state.Reading{}, ctx.Err()
	}
}

func capacity(n uint64) *uint64 { return &n }

func TestPollMergesSources(t *testing.T) {
	a := &fake{engine: "a", reading: state.Reading{
		Pools:  []state.Pool{{Name: state.NVMe}, {Name: state.VRAM, CapacityBytes: capacity(4)}, {Name: state.DRAM}},
		Models: []state.Model{{ID: "m1", State: state.Idle}},
	}}
	b := &fake{engine: "b", reading: state.Reading{
		Pools:  []state.Pool{{Name: state.VRAM, CapacityBytes: capacity(8)}, {Name: state.DRAM, CapacityBytes: capacity(16)}},
		Models: []state.Model{{ID: "m2", State: state.Serving}},
	}}
	broken := &fake{engine: "c", err: errors.New("connection refused")}
	c := New([]Source{a, b, broken}, time.Second)
	c.Poll(context.Background())
	s := c.State()

	if s.Schema != state.Schema {
		t.Errorf("schema %d, expected %d", s.Schema, state.Schema)
	}
	var names []string
	for _, p := range s.Pools {
		names = append(names, p.Name)
	}
	if len(names) != 3 || names[0] != state.VRAM || names[1] != state.DRAM || names[2] != state.NVMe {
		t.Errorf("pools %v, expected vram, dram, nvme", names)
	}
	if *s.Pools[0].CapacityBytes != 4 {
		t.Errorf("vram capacity %d, expected the first source's 4", *s.Pools[0].CapacityBytes)
	}
	if s.Pools[1].CapacityBytes == nil || *s.Pools[1].CapacityBytes != 16 {
		t.Error("dram capacity unknown to the first source not taken from the second")
	}
	if len(s.Models) != 2 || s.Models[0].Engine != "a" || s.Models[1].Endpoint != "http://b" {
		t.Errorf("models not stamped with their source: %+v", s.Models)
	}
	if !s.Sources[0].OK || s.Sources[2].OK || s.Sources[2].Error != "connection refused" {
		t.Errorf("sources %+v, expected a and b ok, c failed", s.Sources)
	}
}

func TestSourcesAreUnreadBeforeFirstPoll(t *testing.T) {
	c := New([]Source{&fake{engine: "a"}}, time.Second)
	if s := c.State(); s.Sources[0].OK || s.Models == nil {
		t.Errorf("before a poll: %+v", s)
	}
}

func TestSlowSourceTimesOut(t *testing.T) {
	slow := &fake{engine: "slow", delay: time.Hour}
	c := New([]Source{slow}, 10*time.Millisecond)
	c.Poll(context.Background())
	if c.State().Sources[0].OK {
		t.Error("a source that never answers reads as ok")
	}
}

func TestRunPollsUntilCancelled(t *testing.T) {
	a := &fake{engine: "a", reading: state.Reading{Models: []state.Model{{ID: "m"}}}}
	c := New([]Source{a}, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx, time.Millisecond); close(done) }()
	deadline := time.Now().Add(time.Second)
	for len(c.State().Models) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done
	if len(c.State().Models) != 1 {
		t.Error("Run never polled")
	}
}

func speed(v float64) *float64 { return &v }

func TestHistoryKeepsTheWindowAndForgetsModelsGone(t *testing.T) {
	fast := state.Model{ID: "fast", DecodeTPS: speed(30)}
	quiet := state.Model{ID: "quiet"}
	a := &fake{engine: "a", reading: state.Reading{Models: []state.Model{fast, quiet}}}
	c := New([]Source{a}, time.Second)
	clock := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return clock }
	for i := 0; i < 3; i++ {
		c.Poll(context.Background())
		clock = clock.Add(time.Minute)
	}
	s := c.State()
	if n := len(s.Models[0].SpeedHistory); n != 2 {
		t.Errorf("fast keeps %d samples after 3 polls a minute apart; expected the 2 within %v", n, historyWindow)
	}
	if s.Models[1].SpeedHistory == nil || len(s.Models[1].SpeedHistory) != 0 {
		t.Errorf("quiet, with no speed, has history %v; expected an empty list", s.Models[1].SpeedHistory)
	}
	a.reading.Models = []state.Model{quiet}
	c.Poll(context.Background())
	if _, kept := c.history[modelKey(state.Model{ID: "fast", Endpoint: "http://a"})]; kept {
		t.Error("history of a model gone is kept")
	}
}
