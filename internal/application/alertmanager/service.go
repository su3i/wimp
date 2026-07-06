package alertmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var client = &http.Client{Timeout: 10 * time.Second}

type alert struct {
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	GeneratorURL string            `json:"generatorURL,omitempty"`
}

// Fire pushes a single firing alert to Alertmanager's /api/v2/alerts endpoint.
// It is a no-op if url is empty. generatorURL is optional - pass "" to omit it (e.g.
// when config.Common().WebUrl isn't configured).
func Fire(url, alertname, severity, instance, category, summary, generatorURL string) error {
	if url == "" {
		return nil
	}

	payload := []alert{
		{
			Labels: map[string]string{
				"alertname": alertname,
				"severity":  severity,
				"instance":  instance,
				"category":  category,
				"source":    "wimp",
			},
			Annotations: map[string]string{
				"summary": summary,
			},
			GeneratorURL: generatorURL,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/api/v2/alerts", url)
	resp, err := client.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("alertmanager: unexpected status %d", resp.StatusCode)
	}
	return nil
}
