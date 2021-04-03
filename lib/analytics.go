package lib

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"cloud.google.com/go/bigquery"
)

// WebVital is a a version of https://web.dev/vitals/.
type WebVital struct {
	// The name of the metric (in acronym form).
	Name string `json:"name"`

	// The current value of the metric.
	Value float64 `json:"value"`

	// The delta between the current value and the last-reported value.
	// On the first report, `delta` and `value` will always be the same.
	Delta float64 `json:"delta"`

	// A unique ID representing this particular metric that's specific to the
	// current page. This ID can be used by an analytics tool to dedupe
	// multiple values sent for the same metric, or to group multiple deltas
	// together and calculate a total.
	ID string `json:"id"`
}

// ParseAnalytics parses a webvitals request body.
func ParseAnalytics(body io.Reader) (*WebVital, error) {
	var data WebVital
	if err := json.NewDecoder(body).Decode(&data); err != nil {
		return nil, fmt.Errorf("could not unmarshal: %w", err)
	}
	return &data, nil
}

// WriteAnalyticsToBigQuery saves a webvital to bq.
func WriteAnalyticsToBigQuery(ctx context.Context, project, dataset, table string, data []*WebVital) error {
	client, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return fmt.Errorf("connecting to bq: %w", err)
	}

	ins := client.Dataset(dataset).Table(table).Inserter()
	if err := ins.Put(ctx, data); err != nil {
		return fmt.Errorf("uploading to bq: %w", err)
	}

	return nil
}
