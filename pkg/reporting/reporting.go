// Package reporting parses payloads sent via the modern Reporting API v1
// (CSP, deprecation, intervention, crash, COEP/COOP, document-policy,
// permissions-policy) and ships them to BigQuery.
package reporting

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
)

// CSPReport carries a Content-Security-Policy violation.
type CSPReport struct {
	Type string        `json:"type"`
	URL  string        `json:"url"`
	Body CSPReportBody `json:"body"`
}

// CSPReportBody is the body of a CSPReport.
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

// DeprecationReport signals use of a deprecated browser API.
type DeprecationReport struct {
	Type string                `json:"type"`
	URL  string                `json:"url"`
	Body DeprecationReportBody `json:"body"`
}

// DeprecationReportBody is the body of a DeprecationReport. It re-declares
// the original CSPReportBody fields so existing BigQuery columns continue
// to type-check; new fields are nullable.
type DeprecationReportBody struct {
	// Fields that existed in BQ from when Body was CSPReportBody (REQUIRED).
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
	// Fields new to DeprecationReportBody — nullable for BQ compatibility.
	// JSON-tagged "-" because they are populated manually in ParseReport
	// from a sibling decode.
	Id                 bigquery.NullString `json:"-"`
	AnticipatedRemoval bigquery.NullString `json:"-"`
	Message            bigquery.NullString `json:"-"`
}

// PermissionsPolicyReport signals a Permissions-Policy violation.
type PermissionsPolicyReport struct {
	Type string                      `json:"type"`
	URL  string                      `json:"url"`
	Body PermissionsPolicyReportBody `json:"body"`
}

// PermissionsPolicyReportBody is the body of a PermissionsPolicyReport.
type PermissionsPolicyReportBody struct {
	FeatureId    string `json:"featureId,omitempty"`
	SourceFile   string `json:"sourceFile,omitempty"`
	LineNumber   int32  `json:"lineNumber,omitempty"`
	ColumnNumber int32  `json:"columnNumber,omitempty"`
	Disposition  string `json:"disposition,omitempty"`
	Message      string `json:"message,omitempty"`
}

// InterventionReport signals that the browser blocked or modified content
// for performance or UX reasons.
type InterventionReport struct {
	Type string                 `json:"type"`
	URL  string                 `json:"url"`
	Body InterventionReportBody `json:"body"`
}

// InterventionReportBody is the body of an InterventionReport.
type InterventionReportBody struct {
	Id           string `json:"id,omitempty"`
	Message      string `json:"message,omitempty"`
	SourceFile   string `json:"sourceFile,omitempty"`
	LineNumber   int32  `json:"lineNumber,omitempty"`
	ColumnNumber int32  `json:"columnNumber,omitempty"`
}

// CrashReport signals a tab crash (OOM or unresponsive page).
type CrashReport struct {
	Type string          `json:"type"`
	URL  string          `json:"url"`
	Body CrashReportBody `json:"body"`
}

// CrashReportBody is the body of a CrashReport.
type CrashReportBody struct {
	Reason string `json:"reason,omitempty"`
	Stack  string `json:"stack,omitempty"`
}

// COEPReport signals a Cross-Origin-Embedder-Policy violation.
type COEPReport struct {
	Type string         `json:"type"`
	URL  string         `json:"url"`
	Body COEPReportBody `json:"body"`
}

// COEPReportBody is the body of a COEPReport.
type COEPReportBody struct {
	Type        string `json:"type,omitempty"`
	BlockedURL  string `json:"blockedURL,omitempty"`
	Destination string `json:"destination,omitempty"`
	Disposition string `json:"disposition,omitempty"`
}

// COOPReport signals a Cross-Origin-Opener-Policy violation.
type COOPReport struct {
	Type string         `json:"type"`
	URL  string         `json:"url"`
	Body COOPReportBody `json:"body"`
}

// COOPReportBody is the body of a COOPReport.
type COOPReportBody struct {
	Type                string `json:"type,omitempty"`
	Property            string `json:"property,omitempty"`
	EffectivePolicy     string `json:"effectivePolicy,omitempty"`
	NextResponseURL     string `json:"nextResponseURL,omitempty"`
	PreviousResponseURL string `json:"previousResponseURL,omitempty"`
}

// DocumentPolicyReport signals a Document-Policy violation.
type DocumentPolicyReport struct {
	Type string                   `json:"type"`
	URL  string                   `json:"url"`
	Body DocumentPolicyReportBody `json:"body"`
}

// DocumentPolicyReportBody is the body of a DocumentPolicyReport.
type DocumentPolicyReportBody struct {
	FeatureId    string `json:"featureId,omitempty"`
	SourceFile   string `json:"sourceFile,omitempty"`
	LineNumber   int32  `json:"lineNumber,omitempty"`
	ColumnNumber int32  `json:"columnNumber,omitempty"`
	Disposition  string `json:"disposition,omitempty"`
	Message      string `json:"message,omitempty"`
}

// SecurityReport is the parsed envelope returned by ParseReport. Exactly
// one of the typed pointers is populated based on the payload's "type"
// field; unknown types are preserved verbatim in RawJSON.
type SecurityReport struct {
	Deprecation       *DeprecationReport       `bigquery:",nullable"`
	CSP               *CSPReport               `bigquery:",nullable"`
	PermissionsPolicy *PermissionsPolicyReport `bigquery:",nullable"`
	Intervention      *InterventionReport      `bigquery:",nullable"`
	Crash             *CrashReport             `bigquery:",nullable"`
	COEP              *COEPReport              `bigquery:",nullable"`
	COOP              *COOPReport              `bigquery:",nullable"`
	DocumentPolicy    *DocumentPolicyReport    `bigquery:",nullable"`

	// ReportType is the value of the payload's "type" field.
	ReportType bigquery.NullString

	// RawJSON preserves the original request body so unknown/future report
	// types remain queryable. Tagged bigquery:"-" because BQ stores the
	// typed columns above; the SQL store keeps RawJSON.
	RawJSON string `bigquery:"-"`

	// Time is when the report was received.
	Time bigquery.NullDateTime
	// Service is the reportd service identifier the report was posted to.
	Service bigquery.NullString
}

// ParseReport decodes a Reporting API v1 payload posted by the browser
// into a SecurityReport. The "type" field of the JSON envelope selects
// which typed pointer is populated; unknown types fall through with only
// RawJSON, ReportType, Time, and Service set.
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
		// Populate NullString fields that json:"-" skips.
		var depJSON struct {
			Body struct {
				Id                 string `json:"id"`
				AnticipatedRemoval string `json:"anticipated_removal"`
				Message            string `json:"message"`
			} `json:"body"`
		}
		if err := json.Unmarshal([]byte(data), &depJSON); err == nil {
			sr.Deprecation.Body.Id = bigquery.NullString{StringVal: depJSON.Body.Id, Valid: depJSON.Body.Id != ""}
			sr.Deprecation.Body.AnticipatedRemoval = bigquery.NullString{StringVal: depJSON.Body.AnticipatedRemoval, Valid: depJSON.Body.AnticipatedRemoval != ""}
			sr.Deprecation.Body.Message = bigquery.NullString{StringVal: depJSON.Body.Message, Valid: depJSON.Body.Message != ""}
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

// WriteReportsToBigQuery streams a single SecurityReport into the
// project.dataset.table BigQuery table.
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

// UpdateReportsBQSchema reconciles project.dataset.table's BigQuery schema
// with the inferred shape of SecurityReport, adding any new columns.
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
