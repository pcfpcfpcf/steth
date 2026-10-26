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

	"github.com/pcfpcfpcf/steth/internal/collector"
	"github.com/pcfpcfpcf/steth/internal/engine/geode"
	"github.com/pcfpcfpcf/steth/internal/engine/llamacpp"
	"github.com/pcfpcfpcf/steth/internal/engine/ollama"
	"github.com/pcfpcfpcf/steth/internal/web"
)

const (
	defaultListen = "127.0.0.1:7474"
	pollInterval  = time.Second
	// Each engine gets most of a poll to answer, so a slow one cannot delay
	// the next.
	readTimeout = 800 * time.Millisecond
)

// engines makes an adapter for each engine a target can name.
var engines = map[string]func(endpoint string, client *http.Client) collector.Source{
	geode.Engine:    func(e string, c *http.Client) collector.Source { return geode.New(e, c) },
	ollama.Engine:   func(e string, c *http.Client) collector.Source { return ollama.New(e, c) },
	llamacpp.Engine: func(e string, c *http.Client) collector.Source { return llamacpp.New(e, c) },
}

// targets collects repeated --target engine=URL flags.
type targets []collector.Source

func (t *targets) String() string { return "" }

func (t *targets) Set(value string) error {
	engine, endpoint, found := strings.Cut(value, "=")
	newSource, known := engines[engine]
	if !found || !known || endpoint == "" {
		return fmt.Errorf("expected ENGINE=URL with ENGINE one of %s, such as geode=http://127.0.0.1:8080", engineNames())
	}
	*t = append(*t, newSource(strings.TrimRight(endpoint, "/"), http.DefaultClient))
	return nil
}

func engineNames() string {
	var names []string
	for name := range engines {
		names = append(names, name)
	}
	slices.Sort(names)
	return strings.Join(names, ", ")
}

func main() {
	var sources targets
	listen := flag.String("listen", defaultListen, "address to serve the page on")
	flag.Var(&sources, "target", "an engine to watch, as ENGINE=URL; repeat for more")
	flag.Parse()
	if len(sources) == 0 {
		fmt.Fprintf(os.Stderr, "steth: name at least one engine with --target ENGINE=URL (ENGINE one of %s)\n", engineNames())
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	c := collector.New(sources, readTimeout)
	go c.Run(ctx, pollInterval)

	server := &http.Server{Addr: *listen, Handler: web.Handler(c.State)}
	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()
	log.Printf("steth: watching %d engine(s) on http://%s", len(sources), *listen)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("steth: cannot serve on %s: %v", *listen, err)
	}
}
