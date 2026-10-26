// Package fetch reads an engine's JSON endpoints.
package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// JSON decodes the body of GET url into `into`, failing on any status but
// 200.
func JSON(ctx context.Context, client *http.Client, url string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d, expected 200", url, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	return nil
}
