// Package collector polls every engine on a fixed schedule and merges what
// they say into one state.State.
package collector

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pcfpcfpcf/steth/internal/state"
)

// Source is one engine endpoint an adapter reads.
type Source interface {
	Engine() string
	Endpoint() string
	Read(ctx context.Context) (state.Reading, error)
}

var errNotRead = errors.New("not read yet")

type Collector struct {
	sources []Source
	timeout time.Duration
	current atomic.Pointer[state.State]
}

// New makes a collector that gives each source `timeout` to answer a poll.
func New(sources []Source, timeout time.Duration) *Collector {
	c := &Collector{sources: sources, timeout: timeout}
	unread := make([]result, len(sources))
	for i := range unread {
		unread[i].err = errNotRead
	}
	c.current.Store(merge(sources, unread))
	return c
}

// State is the box as of the last poll.
func (c *Collector) State() state.State { return *c.current.Load() }

// Run polls now and then every `interval` until ctx ends. A poll that runs
// long delays the next instead of stacking them.
func (c *Collector) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		c.Poll(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

type result struct {
	reading state.Reading
	err     error
}

// Poll reads every source at once and replaces the state with what they
// say. A poll cut short by ctx ending changes nothing: the sources did not
// fail, the caller stopped asking.
func (c *Collector) Poll(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, c.timeout)
	defer cancel()
	results := make([]result, len(c.sources))
	var wg sync.WaitGroup
	for i, source := range c.sources {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reading, err := source.Read(ctx)
			results[i] = result{reading, err}
		}()
	}
	wg.Wait()
	if parent.Err() != nil {
		return
	}
	c.current.Store(merge(c.sources, results))
}

// merge joins the sources' readings: pools by name, the first source to
// report one setting its capacity; models one after another, stamped with
// their source.
func merge(sources []Source, results []result) *state.State {
	s := &state.State{Schema: state.Schema, Pools: []state.Pool{}, Models: []state.Model{}, Sources: []state.Source{}}
	for i, source := range sources {
		r := results[i]
		status := state.Source{Engine: source.Engine(), Endpoint: source.Endpoint(), OK: r.err == nil}
		if r.err != nil {
			status.Error = r.err.Error()
		}
		s.Sources = append(s.Sources, status)
		for _, p := range r.reading.Pools {
			if !slices.ContainsFunc(s.Pools, func(q state.Pool) bool { return q.Name == p.Name }) {
				s.Pools = append(s.Pools, p)
			}
		}
		for _, m := range r.reading.Models {
			m.Engine, m.Endpoint = source.Engine(), source.Endpoint()
			s.Models = append(s.Models, m)
		}
	}
	slices.SortStableFunc(s.Pools, func(a, b state.Pool) int { return poolRank(a.Name) - poolRank(b.Name) })
	return s
}

// poolRank orders the known pools as state.PoolOrder does, any other after.
func poolRank(name string) int {
	if i := slices.Index(state.PoolOrder, name); i >= 0 {
		return i
	}
	return len(state.PoolOrder)
}
