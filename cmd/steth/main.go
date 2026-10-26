// Command steth serves a page that shows what the inference engines on a box
// have loaded, where, and how they are doing.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"time"

	"github.com/pcfpcfpcf/steth/internal/box"
	"github.com/pcfpcfpcf/steth/internal/collector"
	"github.com/pcfpcfpcf/steth/internal/discover"
	"github.com/pcfpcfpcf/steth/internal/engine/geode"
	"github.com/pcfpcfpcf/steth/internal/engine/llamacpp"
	"github.com/pcfpcfpcf/steth/internal/engine/ollama"
	"github.com/pcfpcfpcf/steth/internal/engine/vllm"
	"github.com/pcfpcfpcf/steth/internal/web"
)

const (
	defaultListen = "127.0.0.1:7474"
	pollInterval  = time.Second
	// Each engine gets most of a poll to answer, so a slow one cannot delay
	// the next.
	readTimeout = 800 * time.Millisecond
	// Engines start and stop less often than they are polled.
	discoverInterval = 10 * time.Second
	discoverTimeout  = 5 * time.Second
	// A port that does not answer a probe quickly is not an engine's.
	probeTimeout = 500 * time.Millisecond
)

// version is set by a release build: -ldflags "-X main.version=v0.1.0".
var version = "dev"

// engines is every engine steth reads.
var engines = []discover.Engine{
	{Name: geode.Engine, Runs: geode.Runs, Ports: geode.Ports, Probe: geode.Probe,
		New: func(url string, c *http.Client) collector.Source { return geode.New(url, c) }},
	{Name: ollama.Engine, Runs: ollama.Runs, Ports: ollama.Ports, Probe: ollama.Probe,
		New: func(url string, c *http.Client) collector.Source { return ollama.New(url, c) }},
	{Name: llamacpp.Engine, Runs: llamacpp.Runs, Ports: llamacpp.Ports, Probe: llamacpp.Probe,
		New: func(url string, c *http.Client) collector.Source { return llamacpp.New(url, c) }},
	{Name: vllm.Engine, Runs: vllm.Runs, Ports: vllm.Ports, Probe: vllm.Probe,
		New: func(url string, c *http.Client) collector.Source { return vllm.New(url, c) }},
}

// targets collects repeated --target engine=URL flags.
type targets []collector.Source

func (t *targets) String() string { return "" }

func (t *targets) Set(value string) error {
	name, endpoint, found := strings.Cut(value, "=")
	at := slices.IndexFunc(engines, func(e discover.Engine) bool { return e.Name == name })
	if !found || at < 0 || endpoint == "" {
		return fmt.Errorf("expected ENGINE=URL with ENGINE one of %s, such as ollama=http://192.168.1.20:11434", engineNames())
	}
	// Discovery names this machine 127.0.0.1; the same engine is read once.
	endpoint = strings.Replace(strings.TrimRight(endpoint, "/"), "//localhost", "//127.0.0.1", 1)
	*t = append(*t, engines[at].New(endpoint, http.DefaultClient))
	return nil
}

func engineNames() string {
	var names []string
	for _, e := range engines {
		names = append(names, e.Name)
	}
	return strings.Join(names, ", ")
}

func main() {
	var sources targets
	listen := flag.String("listen", defaultListen, "address to serve the page on")
	flag.Var(&sources, "target", "an engine discovery cannot find, such as one on another machine, as ENGINE=URL; repeat for more")
	find := flag.Bool("discover", true, "find the engines running on this machine")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("steth", version)
		return
	}
	if len(sources) == 0 && !*find {
		fmt.Fprintf(os.Stderr, "steth: with -discover=false, name an engine with --target ENGINE=URL (ENGINE one of %s)\n", engineNames())
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// The box first: the pool sizes it measures take precedence over an
	// engine's own view of them.
	registry := discover.NewRegistry(append([]collector.Source{box.Source{}}, sources...), engines,
		&http.Client{Timeout: probeTimeout})
	if *find {
		refresh(ctx, registry)
		go discoverEvery(ctx, registry)
	}
	c := collector.New(registry.Sources, readTimeout)
	go c.Run(ctx, pollInterval)

	server := &http.Server{Addr: *listen, Handler: web.Handler(c.State)}
	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()
	log.Printf("steth: serving on http://%s", *listen)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("steth: cannot serve on %s: %v", *listen, err)
	}
}

// discoverEvery looks for engines again every discoverInterval until ctx
// ends.
func discoverEvery(ctx context.Context, registry *discover.Registry) {
	ticker := time.NewTicker(discoverInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refresh(ctx, registry)
		}
	}
}

var lastSeen, lastErr string

// refresh looks for engines once, saying what changed.
func refresh(ctx context.Context, registry *discover.Registry) {
	ctx, cancel := context.WithTimeout(ctx, discoverTimeout)
	defer cancel()
	if err := registry.Refresh(ctx, discover.Box); err != nil {
		if err.Error() != lastErr {
			log.Printf("steth: cannot look for engines, reading only --target: %v", err)
			lastErr = err.Error()
		}
		return
	}
	lastErr = ""
	var seen []string
	for _, s := range registry.Sources() {
		if s.Engine() != box.Engine {
			seen = append(seen, s.Engine()+" at "+s.Endpoint())
		}
	}
	if now := strings.Join(seen, ", "); now != lastSeen {
		if now == "" {
			now = "no engine"
		}
		log.Printf("steth: reading %s", now)
		lastSeen = now
	}
}
