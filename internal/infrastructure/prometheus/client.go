// Package prometheus is a minimal shared client for instant PromQL queries against the
// control plane's configured Prometheus instance - used by both the health-check
// alerter and the metrics threshold checker so the HTTP+JSON plumbing isn't duplicated.
package prometheus

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type InstantResult struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Metric map[string]string  `json:"metric"`
			Value  [2]json.RawMessage `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// QueryInstant runs a PromQL instant query against prometheusUrl's /api/v1/query endpoint.
func QueryInstant(prometheusUrl, query string) (*InstantResult, error) {
	u := fmt.Sprintf("%s/api/v1/query?query=%s", prometheusUrl, url.QueryEscape(query))
	resp, err := http.Get(u) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var pr InstantResult
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}
