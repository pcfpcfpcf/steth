package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mime"
	"net"
	"net/http"
	"strings"
)

// The most an action's request body may hold: a chat's whole conversation.
const bodyMax = 1 << 20

// Loopback reports whether a listen address answers only this machine, the
// one place actions are served.
func Loopback(listen string) bool {
	host, _, err := net.SplitHostPort(listen)
	return err == nil && loopbackHost(host)
}

func loopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

// guard answers 403 to a request not from steth's own page on this machine:
// a Host that is not a loopback name (a name rebound to 127.0.0.1 has its
// own), and, for a POST, an Origin other than the page's. A GET needs no
// Origin: without CORS headers no other page can read the answer.
func guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if !loopbackHost(host) {
			fail(w, http.StatusForbidden, fmt.Errorf("actions answer only on a loopback address, not %q", r.Host))
			return
		}
		if r.Method == http.MethodPost && r.Header.Get("Origin") != "http://"+r.Host {
			fail(w, http.StatusForbidden, errors.New("actions answer only steth's own page"))
			return
		}
		next(w, r)
	}
}

// decode reads a JSON body of at most bodyMax into `into`, refusing other
// content types and unknown fields.
func decode(w http.ResponseWriter, r *http.Request, into any) error {
	if media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); media != "application/json" {
		return errors.New("expected a JSON body (Content-Type: application/json)")
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, bodyMax))
	d.DisallowUnknownFields()
	if err := d.Decode(into); err != nil {
		return fmt.Errorf("expected a JSON body of at most %d bytes: %w", bodyMax, err)
	}
	return nil
}

// fail answers {"error": message}.
func fail(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// reply answers `v` as JSON.
func reply(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("answer not sent: %v", err)
	}
}
