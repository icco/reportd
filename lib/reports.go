package lib

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	"google.golang.org/api/iterator"
)

// Report is a simple interface for types exported by ParseReport.
type Report struct {
	ExpectCT *ExpectCTReport `bigquery:",nullable"`
	CSP      *CSPReport      `bigquery:",nullable"`
	ReportTo []*ReportToReport

	// When we recorded this metric.
	Time bigquery.NullDateTime

	// What service this is for.
	Service bigquery.NullString
}

func (r *Report) Validate() error {
	if !r.Service.Valid {
		return fmt.Errorf("service is null")
	}

	if r.Service.StringVal == "" {
		return fmt.Errorf("service is empty")
	}

	return nil
}

// ExpectCTReport is the struct for Expect-CT errors.
type ExpectCTReport struct {
	ExpectCTReport ExpectCTSubReport `json:"expect-ct-report"`
}

// ExpectCTSubReport is the internal datastructure of an ExpectCTReport.
type ExpectCTSubReport struct {
	DateTime                  time.Time `json:"date-time"`
	EffectiveExpirationDate   time.Time `json:"effective-expiration-date"`
	Hostname                  string    `json:"hostname"`
	Port                      int       `json:"port"`
	Scts                      []string  `json:"scts"`
	ServedCertificateChain    []string  `json:"served-certificate-chain"`
	ValidatedCertificateChain []string  `json:"validated-certificate-chain"`
}

// CSPReport is the struct for CSP errors.
// Spec is at https://www.w3.org/TR/CSP3/#violation.
type CSPReport struct {
	CSPReport struct {
		DocumentURI        string `json:"document-uri"`
		Referer            string `json:"referrer"`
		ViolatedDirective  string `json:"violated-directive"`
		EffectiveDirective string `json:"effective-directive"`
		OriginalPolicy     string `json:"original-policy"`
		BlockedURI         string `json:"blocked-uri"`
		StatusCode         int    `json:"status-code"`
		SourceFile         string `json:"source-file"`
		LineNumber         int    `json:"line-number"`
		ColumnNumber       int    `json:"column-number"`
	} `json:"csp-report"`
}

// ReportToReport is the struct for generic reports via the Reporting API.
// TODO: There are multiple ways browsers send the field statuscode!
type ReportToReport struct {
	Type      string `json:"type"`
	Age       int    `json:"age"`
	URL       string `json:"url"`
	UserAgent string `json:"user_agent"`
	Body      struct {
		AnticipatedRemoval float64 `json:"anticipatedRemoval,omitempty"`
		Blocked            string  `json:"blocked,omitempty"`
		BlockedURL         string  `json:"blockedURL,omitempty"`
		ColumnNumber       int64   `json:"columnNumber,omitempty"`
		Directive          string  `json:"directive,omitempty"`
		Disposition        string  `json:"disposition,omitempty"`
		DocumentURL        string  `json:"documentURL,omitempty"`
		EffectiveDirective string  `json:"effectiveDirective,omitempty"`
		ElapsedTime        int64   `json:"elapsed_time,omitempty"`
		ID                 string  `json:"id,omitempty"`
		LineNumber         int64   `json:"lineNumber,omitempty"`
		Message            string  `json:"message,omitempty"`
		Method             string  `json:"method,omitempty"`
		OriginalPolicy     string  `json:"originalPolicy,omitempty"`
		Phase              string  `json:"phase,omitempty"`
		Policy             string  `json:"policy,omitempty"`
		Protocol           string  `json:"protocol,omitempty"`
		Reason             string  `json:"reason,omitempty"`
		Referrer           string  `json:"referrer,omitempty"`
		SamplingFraction   float64 `json:"sampling_fraction,omitempty"`
		ServerIP           string  `json:"server_ip,omitempty"`
		SourceFile         string  `json:"sourceFile,omitempty"`
		Status             int64   `json:"status,omitempty"`
		StatusCode         int64   `json:"status_code,omitempty"`
		Type               string  `json:"type,omitempty"`
	} `json:"body"`
}

// ParseReport takes a content-type header and a body json string and parses it
// into valid Go structs.
func ParseReport(ct, body, srv string) (*Report, error) {
	now := bigquery.NullDateTime{DateTime: civil.DateTimeOf(time.Now()), Valid: true}
	service := bigquery.NullString{StringVal: srv, Valid: true}

	media, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return nil, err
	}

	var r *Report

	switch media {
	case "application/reports+json":
		var data []*ReportToReport
		if err := json.Unmarshal([]byte(body), &data); err != nil {
			return nil, err
		}
		r = &Report{ReportTo: data, Time: now, Service: service}
	case "application/expect-ct-report+json":
		var data ExpectCTReport
		if err := json.Unmarshal([]byte(body), &data); err != nil {
			return nil, err
		}
		r = &Report{ExpectCT: &data, Time: now, Service: service}
	case "application/csp-report":
		var data CSPReport
		if err := json.Unmarshal([]byte(body), &data); err != nil {
			return nil, err
		}
		r = &Report{CSP: &data, Time: now, Service: service}
	default:
		return nil, fmt.Errorf("%q is not a valid content-type", media)
	}

	return r, r.Validate()
}

// UpdateReportsBQSchema updates the bigquery schema if fields are added.
func UpdateReportsBQSchema(ctx context.Context, project, dataset, table string) error {
	client, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return fmt.Errorf("connecting to bq: %w", err)
	}

	t := client.Dataset(dataset).Table(table)
	md, err := t.Metadata(ctx)
	if err != nil {
		return fmt.Errorf("getting table meta: %w", err)
	}

	s, err := getReportSchema()
	if err != nil {
		return fmt.Errorf("infer schema: %w", err)
	}

	if _, err := t.Update(ctx, bigquery.TableMetadataToUpdate{Schema: s}, md.ETag); err != nil {
		return fmt.Errorf("updating table: %w", err)
	}

	return nil
}

func getReportSchema() (bigquery.Schema, error) {
	return bigquery.InferSchema(Report{})
}

// WriteReportToBigQuery saves a copy of a report to BQ.
func WriteReportToBigQuery(ctx context.Context, project, dataset, table string, reports []*Report) error {
	client, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return fmt.Errorf("connecting to bq: %w", err)
	}

	ins := client.Dataset(dataset).Table(table).Inserter()
	if err := ins.Put(ctx, reports); err != nil {
		return fmt.Errorf("uploading to bq: %w", err)
	}

	return nil
}

func GetReports(ctx context.Context, project, dataset, table string) ([]*Report, error) {
	client, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("connecting to bq: %w", err)
	}

	t := client.Dataset(dataset).Table(table)
	q := client.Query(fmt.Sprintf("SELECT * FROM `%s` AS t WHERE DATE(t.Time) = CURRENT_DATE();", t.FullyQualifiedName()))
	it, err := q.Read(ctx)
	if err != nil {
		return nil, err
	}

	var ret []*Report
	for {
		var r Report
		err := it.Next(&r)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("couldn't get Report: %w", err)
		}

		ret = append(ret, &r)
	}

	return ret, nil
}
