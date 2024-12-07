package reporting

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
)

type CSPReport struct {
	Type string        `json:"type"`
	URL  string        `json:"url"`
	Body CSPReportBody `json:"body"`
}

type CSPReportBody struct {
	// The URI of the document in which the violation occurred.
	DocumentUri string `protobuf:"bytes,4,opt,name=document_uri,json=documentUri,proto3" json:"document_uri,omitempty"`
	// The referrer of the document in which the violation occurred.
	Referrer string `protobuf:"bytes,5,opt,name=referrer,proto3" json:"referrer,omitempty"`
	// The URI of the resource that was blocked from loading by the
	// Content Security Policy. If the blocked URI is from a different
	// origin than the document-uri, then the blocked URI is truncated
	// to contain just the scheme, host, and port.
	BlockedUri string `protobuf:"bytes,6,opt,name=blocked_uri,json=blockedUri,proto3" json:"blocked_uri,omitempty"`
	// The name of the policy section that was violated.
	ViolatedDirective string `protobuf:"bytes,7,opt,name=violated_directive,json=violatedDirective,proto3" json:"violated_directive,omitempty"`
	// The name of the policy directive that was violated.
	EffectiveDirective string `protobuf:"bytes,15,opt,name=effective_directive,json=effectiveDirective,proto3" json:"effective_directive,omitempty"`
	// The original policy as specified by the CSP HTTP header:
	// Content-Security-Policy, X-Content-Security-Policy (IE),
	// X-Webkit-CSP (old Safari, old Chrome).
	OriginalPolicy string `protobuf:"bytes,8,opt,name=original_policy,json=originalPolicy,proto3" json:"original_policy,omitempty"`
	// The URL of the resource where the violation occurred.
	SourceFile string `protobuf:"bytes,9,opt,name=source_file,json=sourceFile,proto3" json:"source_file,omitempty"`
	// The line number in source-file on which the violation occurred, 1-based.
	LineNumber int32 `protobuf:"varint,31,opt,name=line_number,json=lineNumber,proto3" json:"line_number,omitempty"`
	// The column number in source-file on which the violation occurred, 1-based.
	ColumnNumber int32 `protobuf:"varint,32,opt,name=column_number,json=columnNumber,proto3" json:"column_number,omitempty"`
	// A snippet of the rejected script (first 40 bytes).
	ScriptSample string `protobuf:"bytes,21,opt,name=script_sample,json=scriptSample,proto3" json:"script_sample,omitempty"`
}

type DeprecationReport struct {
	Type string        `json:"type"`
	URL  string        `json:"url"`
	Body CSPReportBody `json:"body"`
}

type DeprecationReportBody struct {
	// name of API, e.g. websql
	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	// YYYY-MM-DD date format, e.g. "2020-01-01"
	AnticipatedRemoval string `protobuf:"bytes,2,opt,name=anticipated_removal,json=anticipatedRemoval,proto3" json:"anticipated_removal,omitempty"`
	// free form text, e.g. "WebSQL is deprecated and will be removed in Chrome 97 around January 2020"
	Message string `protobuf:"bytes,3,opt,name=message,proto3" json:"message,omitempty"`
	// where the call to the deprecated API happened, e.g. https://example.com/index.js
	SourceFile string `protobuf:"bytes,4,opt,name=source_file,json=sourceFile,proto3" json:"source_file,omitempty"`
	// 1-based
	LineNumber int32 `protobuf:"varint,5,opt,name=line_number,json=lineNumber,proto3" json:"line_number,omitempty"`
	// 1-based
	ColumnNumber int32 `protobuf:"varint,6,opt,name=column_number,json=columnNumber,proto3" json:"column_number,omitempty"`
}

type SecurityReport struct {
	Deprecation *DeprecationReport `bigquery:",nullable"`
	CSP         *CSPReport         `bigquery:",nullable"`

	// When we recorded this metric.
	Time bigquery.NullDateTime

	// What service this is for.
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

	switch tmp.Type {
	case "csp-violation":
		if err := json.Unmarshal([]byte(data), &sr.CSP); err != nil {
			return nil, err
		}
	case "deprecation":
		if err := json.Unmarshal([]byte(data), &sr.Deprecation); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown report type: %s", tmp.Type)
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
