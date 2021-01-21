package lib

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/bigquery"
)

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

	// Any performance entries used in the metric value calculation.
	// Note, entries will be added to the array as the value changes.
	//
	// TODO: Find an example of this, and implement struct.
	Entries []interface{} `json:"entries"`
}

func ParseAnalytics(body []byte) (*WebVital, error) {
	var data WebVital
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("could not unmarshal: %w", err)
	}
	return &data, nil
}

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
