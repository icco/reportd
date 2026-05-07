// Package analytics models Web Vitals measurements and ships them to BigQuery.
package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
)

// WebVital is one Web Vitals measurement; see https://web.dev/vitals/ and
// https://nextjs.org/docs/app/guides/analytics#build-your-own.
type WebVital struct {
	Name    string              `json:"name"`
	Value   float64             `json:"value"`
	Delta   float64             `json:"delta"`
	ID      string              `json:"id"`
	Label   bigquery.NullString `json:"label"`
	Time    bigquery.NullDateTime
	Service bigquery.NullString
}

// Validate returns an error if Service is unset or empty.
func (wv *WebVital) Validate() error {
	if !wv.Service.Valid {
		return fmt.Errorf("service is null")
	}

	if wv.Service.StringVal == "" {
		return fmt.Errorf("service is empty")
	}

	return nil
}

// ParseAnalytics decodes a Web Vitals JSON payload, stamping it with the
// current time and service.
func ParseAnalytics(body, service string) (*WebVital, error) {
	now := civil.DateTimeOf(time.Now())
	var data WebVital
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return nil, fmt.Errorf("could not unmarshal: %w", err)
	}

	data.Time = bigquery.NullDateTime{DateTime: now, Valid: true}
	data.Service = bigquery.NullString{StringVal: service, Valid: true}

	return &data, nil
}

// UpdateAnalyticsBQSchema reconciles project.dataset.table's schema with
// the inferred shape of WebVital.
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

// WriteAnalyticsToBigQuery validates data and streams it into
// project.dataset.table.
func WriteAnalyticsToBigQuery(ctx context.Context, project, dataset, table string, data []*WebVital) error {
	for _, d := range data {
		if err := d.Validate(); err != nil {
			return fmt.Errorf("validating data: %w", err)
		}
	}

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
