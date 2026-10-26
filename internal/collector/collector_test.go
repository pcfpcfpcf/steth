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
		Pools:  []state.Pool{{Name: state.NVMe}, {Name: state.VRAM, CapacityBytes: capacity(4)}},
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
