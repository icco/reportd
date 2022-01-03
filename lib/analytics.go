package lib

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
)

// WebVital is a a version of https://web.dev/vitals/.
//
// See also https://nextjs.org/docs/advanced-features/measuring-performance#build-your-own.
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

	// Type of metric (web-vital or custom).
	Label bigquery.NullString `json:"label"`

	// When we recorded this metric.
	Time bigquery.NullDateTime

	// What service this is for.
	Service bigquery.NullString
}

// ParseAnalytics parses a webvitals request body.
func ParseAnalytics(body, service string) (*WebVital, error) {
	now := civil.DateTimeOf(time.Now())
	var data WebVital
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return nil, fmt.Errorf("could not unmarshal: %w", err)
	}

	data.Time = bigquery.NullDateTime{DateTime: now, Valid: true}
	data.Service = service

	return &data, nil
}

// UpdateAnalyticsBQSchema updates the bigquery schema if fields are added.
func UpdateAnalyticsBQSchema(ctx context.Context, project, dataset, table string) error {
	client, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return fmt.Errorf("connecting to bq: %w", err)
	}

	t := client.Dataset(dataset).Table(table)
	md, err := t.Metadata(ctx)
	if err != nil {
		return fmt.Errorf("getting table meta: %w", err)
	}

	s, err := getAnalyticsSchema()
	if err != nil {
		return fmt.Errorf("infer schema: %w", err)
	}

	if _, err := t.Update(ctx, bigquery.TableMetadataToUpdate{Schema: s}, md.ETag); err != nil {
		return fmt.Errorf("updating table: %w", err)
	}

	return nil
}

func getAnalyticsSchema() (bigquery.Schema, error) {
	return bigquery.InferSchema(WebVital{})
}

// WriteAnalyticsToBigQuery saves a webvital to bq.
func WriteAnalyticsToBigQuery(ctx context.Context, project, dataset, table string, data []*WebVital) error {
	client, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return fmt.Errorf("connecting to bq: %w", err)
	}

	t := client.Dataset(dataset).Table(table)
	ins := t.Inserter()
	if err := ins.Put(ctx, data); err != nil {
		return fmt.Errorf("uploading to bq: %w", err)
	}

	return nil
}
