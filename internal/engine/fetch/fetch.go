// Package fetch reads an engine's endpoints and posts to its controls.
package fetch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// How much of an error answer is read, and how much of one that is not
// JSON goes into the error.
const (
	errorBodyBytes = 64 << 10
	snippetBytes   = 200
)

// StatusError is an answer with a status the caller did not accept.
type StatusError struct {
	Method, URL string
	Status      int
	// What the engine said went wrong, or the start of its answer.
	Message string
}

func (e *StatusError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("%s %s: status %d", e.Method, e.URL, e.Status)
	}
	return fmt.Sprintf("%s %s: status %d: %s", e.Method, e.URL, e.Status, e.Message)
}

// Body returns the body of GET url, failing on any status but 200.
func Body(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	status, body, err := send(client, req)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, &StatusError{Method: http.MethodGet, URL: url, Status: status, Message: "expected 200"}
	}
	return body, nil
}

// JSON decodes the body of GET url into `into`.
func JSON(ctx context.Context, client *http.Client, url string, into any) error {
	body, err := Body(ctx, client, url)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, into); err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	return nil
}

// Post POSTs `body` as JSON to url, or nothing when it is nil, accepts any
// 2xx, and decodes the answer into `into` when that is not nil. Only ctx
// bounds it, not `client`'s timeout: a POST is an action, and a load reads
// a model's weights, far longer than a read is given.
func Post(ctx context.Context, client *http.Client, url string, body, into any) error {
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("POST %s: %w", url, err)
		}
		payload = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, payload)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	unbounded := *client
	unbounded.Timeout = 0
	_, answer, err := send(&unbounded, req)
	if err != nil || into == nil {
		return err
	}
	if err := json.Unmarshal(answer, into); err != nil {
		return fmt.Errorf("POST %s: %w", url, err)
	}
	return nil
}

// send does `req` and returns its status and body, failing on a status
// outside 2xx.
func send(client *http.Client, req *http.Request) (int, []byte, error) {
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return 0, nil, Refusal(resp)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("%s %s: %w", req.Method, req.URL, err)
	}
	return resp.StatusCode, body, nil
}

// Refusal is the error for an answer the caller did not accept, with the
// message the engine put in it. A body cut short still says what went
// wrong; the status is the error.
func Refusal(resp *http.Response) *StatusError {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, errorBodyBytes))
	return &StatusError{Method: resp.Request.Method, URL: resp.Request.URL.String(), Status: resp.StatusCode, Message: engineMessage(body)}
}

// engineMessage is the error an engine put in its answer: OpenAI's
// {"error":{"message":...}} (llama.cpp, vLLM), {"error":"..."} (Ollama), or
// else the start of the body.
func engineMessage(body []byte) string {
	var nested struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &nested) == nil && nested.Error.Message != "" {
		return nested.Error.Message
	}
	var flat struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &flat) == nil && flat.Error != "" {
		return flat.Error
	}
	snippet := bytes.TrimSpace(body)
	if len(snippet) > snippetBytes {
		return string(snippet[:snippetBytes]) + "..."
	}
	return string(snippet)
}
