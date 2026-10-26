// Package fetch reads an engine's endpoints.
package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Body returns the body of GET url, failing on any status but 200.
func Body(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d, expected 200", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
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
