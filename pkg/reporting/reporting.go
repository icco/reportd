package reporting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	"github.com/icco/reportd/pkg/analytics"
	"google.golang.org/api/iterator"
)

type CSPReport struct {
	Type string        `json:"type"`
	URL  string        `json:"url"`
	Body CSPReportBody `json:"body"`
}

type CSPReportBody struct {
	DocumentUri        string `json:"document_uri,omitempty"`
	Referrer           string `json:"referrer,omitempty"`
	BlockedUri         string `json:"blocked_uri,omitempty"`
	ViolatedDirective  string `json:"violated_directive,omitempty"`
	EffectiveDirective string `json:"effective_directive,omitempty"`
	OriginalPolicy     string `json:"original_policy,omitempty"`
	SourceFile         string `json:"source_file,omitempty"`
	LineNumber         int32  `json:"line_number,omitempty"`
	ColumnNumber       int32  `json:"column_number,omitempty"`
	ScriptSample       string `json:"script_sample,omitempty"`
}

type DeprecationReport struct {
	Type string              `json:"type"`
	URL  string              `json:"url"`
	Body DeprecationReportBody `json:"body"`
}

type DeprecationReportBody struct {
	Id                 string `json:"id,omitempty"`
	AnticipatedRemoval string `json:"anticipated_removal,omitempty"`
	Message            string `json:"message,omitempty"`
	SourceFile         string `json:"source_file,omitempty"`
	LineNumber         int32  `json:"line_number,omitempty"`
	ColumnNumber       int32  `json:"column_number,omitempty"`
}

type PermissionsPolicyReport struct {
	Type string                        `json:"type"`
	URL  string                        `json:"url"`
	Body PermissionsPolicyReportBody   `json:"body"`
}

type PermissionsPolicyReportBody struct {
	FeatureId    string `json:"featureId,omitempty"`
	SourceFile   string `json:"sourceFile,omitempty"`
	LineNumber   int32  `json:"lineNumber,omitempty"`
	ColumnNumber int32  `json:"columnNumber,omitempty"`
	Disposition  string `json:"disposition,omitempty"`
	Message      string `json:"message,omitempty"`
}

type InterventionReport struct {
	Type string                   `json:"type"`
	URL  string                   `json:"url"`
	Body InterventionReportBody   `json:"body"`
}

type InterventionReportBody struct {
	Id           string `json:"id,omitempty"`
	Message      string `json:"message,omitempty"`
	SourceFile   string `json:"sourceFile,omitempty"`
	LineNumber   int32  `json:"lineNumber,omitempty"`
	ColumnNumber int32  `json:"columnNumber,omitempty"`
}

type CrashReport struct {
	Type string          `json:"type"`
	URL  string          `json:"url"`
	Body CrashReportBody `json:"body"`
}

type CrashReportBody struct {
	Reason string `json:"reason,omitempty"`
	Stack  string `json:"stack,omitempty"`
}

type COEPReport struct {
	Type string         `json:"type"`
	URL  string         `json:"url"`
	Body COEPReportBody `json:"body"`
}

type COEPReportBody struct {
	Type        string `json:"type,omitempty"`
	BlockedURL  string `json:"blockedURL,omitempty"`
	Destination string `json:"destination,omitempty"`
	Disposition string `json:"disposition,omitempty"`
}

type COOPReport struct {
	Type string         `json:"type"`
	URL  string         `json:"url"`
	Body COOPReportBody `json:"body"`
}

type COOPReportBody struct {
	Type                string `json:"type,omitempty"`
	Property            string `json:"property,omitempty"`
	EffectivePolicy     string `json:"effectivePolicy,omitempty"`
	NextResponseURL     string `json:"nextResponseURL,omitempty"`
	PreviousResponseURL string `json:"previousResponseURL,omitempty"`
}

type DocumentPolicyReport struct {
	Type string                     `json:"type"`
	URL  string                     `json:"url"`
	Body DocumentPolicyReportBody   `json:"body"`
}

type DocumentPolicyReportBody struct {
	FeatureId    string `json:"featureId,omitempty"`
	SourceFile   string `json:"sourceFile,omitempty"`
	LineNumber   int32  `json:"lineNumber,omitempty"`
	ColumnNumber int32  `json:"columnNumber,omitempty"`
	Disposition  string `json:"disposition,omitempty"`
	Message      string `json:"message,omitempty"`
}

type SecurityReport struct {
	Deprecation       *DeprecationReport       `bigquery:",nullable"`
	CSP               *CSPReport               `bigquery:",nullable"`
	PermissionsPolicy *PermissionsPolicyReport  `bigquery:",nullable"`
	Intervention      *InterventionReport       `bigquery:",nullable"`
	Crash             *CrashReport              `bigquery:",nullable"`
	COEP              *COEPReport               `bigquery:",nullable"`
	COOP              *COOPReport               `bigquery:",nullable"`
	DocumentPolicy    *DocumentPolicyReport     `bigquery:",nullable"`

	ReportType bigquery.NullString

	// Raw JSON for unknown/forward-compatible types.
	RawJSON string `bigquery:"-"`

	Time    bigquery.NullDateTime
	Service bigquery.NullString
}

func ParseReport(data, srv string) (*SecurityReport, error) {
	sr := &SecurityReport{
		Time:    bigquery.NullDateTime{DateTime: civil.DateTimeOf(time.Now()), Valid: true},
		Service: bigquery.NullString{StringVal: srv, Valid: true},
	}

	tmp := struct {
		Type string `json:"type"`
	}{}

	if err := json.Unmarshal([]byte(data), &tmp); err != nil {
		return nil, err
	}

	sr.ReportType = bigquery.NullString{StringVal: tmp.Type, Valid: true}
	sr.RawJSON = data

	switch tmp.Type {
	case "csp-violation":
		if err := json.Unmarshal([]byte(data), &sr.CSP); err != nil {
			return nil, err
		}
	case "deprecation":
		if err := json.Unmarshal([]byte(data), &sr.Deprecation); err != nil {
			return nil, err
		}
	case "permissions-policy-violation":
		if err := json.Unmarshal([]byte(data), &sr.PermissionsPolicy); err != nil {
			return nil, err
		}
	case "intervention":
		if err := json.Unmarshal([]byte(data), &sr.Intervention); err != nil {
			return nil, err
		}
	case "crash":
		if err := json.Unmarshal([]byte(data), &sr.Crash); err != nil {
			return nil, err
		}
	case "coep":
		if err := json.Unmarshal([]byte(data), &sr.COEP); err != nil {
			return nil, err
		}
	case "coop":
		if err := json.Unmarshal([]byte(data), &sr.COOP); err != nil {
			return nil, err
		}
	case "document-policy-violation":
		if err := json.Unmarshal([]byte(data), &sr.DocumentPolicy); err != nil {
			return nil, err
		}
	default:
		// Forward-compatible: store raw JSON for unknown types.
	}

	return sr, nil
}

func WriteReportsToBigQuery(ctx context.Context, project, dataset, table string, report *SecurityReport) error {
	bq, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return fmt.Errorf("connecting to bq: %w", err)
	}

	ins := bq.Dataset(dataset).Table(table).Inserter()
	if err := ins.Put(ctx, report); err != nil {
		return fmt.Errorf("uploading to bq: %w", err)
	}
	return nil
}

func GetReportCounts(ctx context.Context, site, project, dataset, table string) ([]*analytics.WebVitalSummary, error) {
	client, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("connecting to bq: %w", err)
	}

	t := client.Dataset(dataset).Table(table)
	tableID, err := t.Identifier(bigquery.StandardSQLID)
	if err != nil {
		return nil, fmt.Errorf("getting table id: %w", err)
	}
	query := fmt.Sprintf(
		"SELECT DATE(Time) AS Day, Service, CAST(COUNT(*) as FLOAT64) AS Value "+
			"FROM `%s` "+
			"WHERE Service = @site AND Time >= DATE_SUB(CURRENT_DATE(), INTERVAL 3 MONTH) "+
			"GROUP BY 1, 2 "+
			"ORDER BY Day DESC;",
		tableID,
	)

	q := client.Query(query)
	q.Parameters = []bigquery.QueryParameter{
		{Name: "site", Value: site},
	}
	it, err := q.Read(ctx)
	if err != nil {
		return nil, err
	}

	var ret []*analytics.WebVitalSummary
	for {
		var r analytics.WebVitalSummary
		err := it.Next(&r)
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("couldn't get WebVitalSummary: %w", err)
		}

		r.Name = "EndpointSecurityReportCount"

		ret = append(ret, &r)
	}

	return ret, nil
}

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
	return bigquery.InferSchema(SecurityReport{})
}
