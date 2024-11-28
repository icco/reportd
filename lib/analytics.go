package lib

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	"google.golang.org/api/iterator"
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

type WebVitalSummary struct {
	// The name of the metric (in acronym form).
	Name string `json:"name"`

	// The current value of the metric.
	Value float64 `json:"value"`

	Service string `json:"service"`

	Date time.Time `json:"date"`
}

func (wv *WebVital) Validate() error {
	if !wv.Service.Valid {
		return fmt.Errorf("service is null")
	}

	if wv.Service.StringVal == "" {
		return fmt.Errorf("service is empty")
	}

	return nil
}

// ParseAnalytics parses a webvitals request body.
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

func GetAnalytics(ctx context.Context, site, project, dataset, table string) ([]*WebVitalSummary, error) {
	client, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("connecting to bq: %w", err)
	}

	t := client.Dataset(dataset).Table(table)
	query := fmt.Sprintf(
		"SELECT DATE(Time) AS Day, Service, Name, AVG(Value) AS AverageValue "+
			"FROM `%s` "+
			"WHERE Service = @site AND Time >= DATE_SUB(CURRENT_DATE(), INTERVAL 24 MONTH) "+
			"GROUP BY 1, 2, 3 "+
			"ORDER BY Day DESC;",
		t.FullyQualifiedName())
	q := client.Query(query)
	q.Parameters = []bigquery.QueryParameter{
		{Name: "site", Value: site},
	}
	it, err := q.Read(ctx)
	if err != nil {
		return nil, err
	}

	var ret []*WebVitalSummary
	for {
		var wv WebVitalSummary
		err := it.Next(&wv)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("couldn't get WebVitalSummary: %w", err)
		}

		ret = append(ret, &wv)
	}

	return ret, nil
}

func GetAnalyticsServices(ctx context.Context, project, dataset, table string) ([]string, error) {
	client, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("connecting to bq: %w", err)
	}

	t := client.Dataset(dataset).Table(table)
	q := client.Query(fmt.Sprintf("SELECT DISTINCT Service FROM `%s` WHERE Service IS NOT NULL;", t.FullyQualifiedName()))
	it, err := q.Read(ctx)
	if err != nil {
		return nil, err
	}

	var ret []string
	for {
		var s string
		err := it.Next(&s)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("couldn't get Services: %w", err)
		}

		ret = append(ret, s)
	}

	return ret, nil
}
