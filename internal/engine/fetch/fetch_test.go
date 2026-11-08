package fetch

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// answer serves every request with `status` and `body`, keeping the last
// request's method, content type and body.
func answer(t *testing.T, status int, body string) (*httptest.Server, *http.Request, *string) {
	got := &http.Request{}
	sent := new(string)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*got = *r
		read, _ := io.ReadAll(r.Body)
		*sent = string(read)
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	return server, got, sent
}

func TestPostSendsJSONAndDecodesTheAnswer(t *testing.T) {
	server, got, sent := answer(t, http.StatusOK, `{"success":true}`)
	var into struct {
		Success bool `json:"success"`
	}
	err := Post(context.Background(), server.Client(), server.URL+"/models/load", map[string]string{"model": "m"}, &into)
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != http.MethodPost || got.Header.Get("Content-Type") != "application/json" || *sent != `{"model":"m"}` {
		t.Errorf("sent %s %q with %q", got.Method, *sent, got.Header.Get("Content-Type"))
	}
	if !into.Success {
		t.Error("answer not decoded")
	}
}

func TestPostWithoutBodyOrAnswer(t *testing.T) {
	server, got, sent := answer(t, http.StatusNoContent, "")
	if err := Post(context.Background(), server.Client(), server.URL+"/wake_up", nil, nil); err != nil {
		t.Fatal(err)
	}
	if *sent != "" || got.Header.Get("Content-Type") != "" {
		t.Errorf("sent %q with %q; expected nothing", *sent, got.Header.Get("Content-Type"))
	}
}

func TestPostErrorSaysWhatTheEngineSaid(t *testing.T) {
	long := strings.Repeat("x", 300)
	for name, c := range map[string]struct {
		status int
		body   string
		want   string
	}{
		"openai": {400, `{"error":{"code":400,"message":"model is already running","type":"invalid_request_error"}}`,
			"status 400: model is already running"},
		"ollama":  {404, `{"error":"model 'nosuch:1b' not found"}`, "status 404: model 'nosuch:1b' not found"},
		"fastapi": {404, `{"detail":"Not Found"}`, `status 404: {"detail":"Not Found"}`},
		"long":    {502, long, "status 502: " + long[:snippetBytes] + "..."},
		"no body": {500, "", "status 500"},
	} {
		t.Run(name, func(t *testing.T) {
			server, _, _ := answer(t, c.status, c.body)
			err := Post(context.Background(), server.Client(), server.URL+"/x", nil, nil)
			want := "POST " + server.URL + "/x: " + c.want
			if err == nil || err.Error() != want {
				t.Fatalf("error %v; expected %q", err, want)
			}
			var status *StatusError
			if !errors.As(err, &status) || status.Status != c.status {
				t.Errorf("error %#v; expected a StatusError with status %d", err, c.status)
			}
		})
	}
}

// A load outlasts the client's read timeout; only its context ends it.
func TestPostOutlastsTheClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	t.Cleanup(server.Close)
	client := &http.Client{Timeout: 20 * time.Millisecond}
	if err := Post(context.Background(), client, server.URL, nil, nil); err != nil {
		t.Errorf("a slow load failed: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := Post(ctx, client, server.URL, nil, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a load past its context: %v, expected the deadline", err)
	}
}

func TestBodyAcceptsOnly200(t *testing.T) {
	server, _, _ := answer(t, http.StatusAccepted, "{}")
	if _, err := Body(context.Background(), server.Client(), server.URL); err == nil {
		t.Error("expected an error for a 202")
	}
}
