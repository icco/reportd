package lib

import (
	"context"
	"fmt"
	"log"
	"mime"

	"cloud.google.com/go/bigquery"
	"github.com/pkg/errors"
)

type WebVital struct {
	// The name of the metric (in acronym form).
	Name string `json:"name"`

	// The current value of the metric.
	Value float64 `json:"value"`

	// The delta between the current value and the last-reported value.
	// On the first report, `delta` and `value` will always be the same.
	delta float64 `json:"delta"`

	// A unique ID representing this particular metric that's specific to the
	// current page. This ID can be used by an analytics tool to dedupe
	// multiple values sent for the same metric, or to group multiple deltas
	// together and calculate a total.
	ID string `json:"id"`

	// Any performance entries used in the metric value calculation.
	// Note, entries will be added to the array as the value changes.
	Entries []interface{} `json:"entries"`
}

func ParseAnalytics(ct, body string) (*WebVital, error) {
	media, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return nil, err
	}
	log.Printf("media: %+v", media)

	return nil, fmt.Errorf("could not parse")
}

func WriteAnalyticsToBigQuery(ctx context.Context, project, dataset, table string, data []*WebVital) error {
	client, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return errors.Wrap(err, "connecting to bq")
	}

	ins := client.Dataset(dataset).Table(table).Inserter()
	if err := ins.Put(ctx, data); err != nil {
		return errors.Wrap(err, "uploading to bq")
	}

	return nil
}
