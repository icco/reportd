// Package reportto parses legacy Report-To payloads (CSP, Expect-CT, and
// the original Reporting API draft) and ships them to BigQuery.
package reportto

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
)

// Content-Type values accepted by ParseReport.
const (
	ContentTypeReports        = "application/reports+json"
	ContentTypeExpectCTReport = "application/expect-ct-report+json"
	ContentTypeCSPReport      = "application/csp-report"
)

// Report is the parsed envelope returned by ParseReport; exactly one of
// ExpectCT, CSP, or ReportTo is populated.
type Report struct {
	ExpectCT *ExpectCTReport `bigquery:",nullable"`
	CSP      *CSPReport      `bigquery:",nullable"`
	ReportTo []*ReportToReport

	Time    bigquery.NullDateTime
	Service bigquery.NullString
}

// Validate returns an error if Service is unset or empty.
func (r *Report) Validate() error {
	if !r.Service.Valid {
		return fmt.Errorf("service is null")
	}
	if r.Service.StringVal == "" {
		return fmt.Errorf("service is empty")
	}
	return nil
}

// ExpectCTReport carries an Expect-CT (Certificate Transparency) violation.
type ExpectCTReport struct {
	ExpectCTReport ExpectCTSubReport `json:"expect-ct-report"`
}

// ExpectCTSubReport is the inner payload of an ExpectCTReport.
type ExpectCTSubReport struct {
	DateTime                  time.Time `json:"date-time"`
	EffectiveExpirationDate   time.Time `json:"effective-expiration-date"`
	Hostname                  string    `json:"hostname"`
	Port                      int       `json:"port"`
	Scts                      []string  `json:"scts"`
	ServedCertificateChain    []string  `json:"served-certificate-chain"`
	ValidatedCertificateChain []string  `json:"validated-certificate-chain"`
}

// CSPReport carries a Content-Security-Policy violation; see
// https://www.w3.org/TR/CSP3/#violation.
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

// ReportToReport is one entry of an application/reports+json payload.
// Body is a superset of fields observed across browsers.
//
// TODO: browsers send status under multiple names — normalize.
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

// ParseReport decodes body according to the Content-Type ct (one of the
// ContentType* constants), scopes it to service srv, and validates it.
func ParseReport(ct, body, srv string) (*Report, error) {
	now := bigquery.NullDateTime{DateTime: civil.DateTimeOf(time.Now()), Valid: true}
	service := bigquery.NullString{StringVal: srv, Valid: true}

	media, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return nil, err
	}

	var r *Report

	switch media {
	case ContentTypeReports:
		var data []*ReportToReport
		if err := json.Unmarshal([]byte(body), &data); err != nil {
			return nil, err
		}
		r = &Report{ReportTo: data, Time: now, Service: service}
	case ContentTypeExpectCTReport:
		var data ExpectCTReport
		if err := json.Unmarshal([]byte(body), &data); err != nil {
			return nil, err
		}
		r = &Report{ExpectCT: &data, Time: now, Service: service}
	case ContentTypeCSPReport:
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

// UpdateReportsBQSchema reconciles project.dataset.table's schema with
// the inferred shape of Report.
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

// WriteReportToBigQuery streams reports into project.dataset.table.
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
