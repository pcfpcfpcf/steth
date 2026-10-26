// Command mockengine answers as an engine from recorded responses, for
// working on the page without running the engine:
//
//	go run ./cmd/mockengine -listen 127.0.0.1:19931 \
//	    /props=internal/engine/llamacpp/testdata/router.props.json \
//	    /v1/models=internal/engine/llamacpp/testdata/router.models.json
//
// then `steth --target llamacpp=http://127.0.0.1:19931`. A path with a query
// answers only that query. Files are read again at every request, so editing
// one changes the answer.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:19931", "address to answer on")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: mockengine [-listen ADDR] PATH=FILE ...")
		flag.PrintDefaults()
	}
	flag.Parse()
	routes := map[string]string{}
	for _, arg := range flag.Args() {
		// The last "=": a path's query has its own.
		at := strings.LastIndex(arg, "=")
		path, file := arg[:max(at, 0)], arg[at+1:]
		if at < 0 || !strings.HasPrefix(path, "/") {
			log.Fatalf("mockengine: expected PATH=FILE such as /props=props.json, got %q", arg)
		}
		if _, err := os.Stat(file); err != nil {
			log.Fatalf("mockengine: %v", err)
		}
		routes[path] = file
	}
	if len(routes) == 0 {
		flag.Usage()
		os.Exit(2)
	}
	log.Printf("mockengine: answering %d paths on http://%s", len(routes), *listen)
	log.Fatal(http.ListenAndServe(*listen, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file, ok := routes[r.URL.RequestURI()]
		if !ok {
			file, ok = routes[r.URL.Path]
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, file)
	})))
}
