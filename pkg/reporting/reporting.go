package reporting

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/icco/gutil/logging"
)

var (
	service = "reportd"
	log     = logging.Must(logging.NewLogger(service))
)

type CspReport struct {
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

// Policy disposition
type SecurityReport_Disposition int32

const (
	SecurityReport_DISPOSITION_UNKNOWN SecurityReport_Disposition = 0
	SecurityReport_REPORTING           SecurityReport_Disposition = 1
	SecurityReport_ENFORCED            SecurityReport_Disposition = 2
)

// Enum value maps for SecurityReport_Disposition.
var (
	SecurityReport_Disposition_name = map[int32]string{
		0: "DISPOSITION_UNKNOWN",
		1: "REPORTING",
		2: "ENFORCED",
	}
	SecurityReport_Disposition_value = map[string]int32{
		"DISPOSITION_UNKNOWN": 0,
		"REPORTING":           1,
		"ENFORCED":            2,
	}
)

func (x SecurityReport_Disposition) Enum() *SecurityReport_Disposition {
	p := new(SecurityReport_Disposition)
	*p = x
	return p
}

type SecurityReport struct {

	// This report's checksum is computed according to the subtype of the
	// report. Used for deduplication.
	ReportChecksum string `protobuf:"bytes,1,opt,name=report_checksum,json=reportChecksum,proto3" json:"report_checksum,omitempty"`
	// When was this report generated? (milliseconds)
	ReportTime int64 `protobuf:"varint,2,opt,name=report_time,json=reportTime,proto3" json:"report_time,omitempty"`
	// Number of times we saw this report (always 1 until aggregation happens)
	ReportCount int64 `protobuf:"varint,3,opt,name=report_count,json=reportCount,proto3" json:"report_count,omitempty"`
	// Unparsed UA + parsed browser name and major version
	UserAgent           string                     `protobuf:"bytes,4,opt,name=user_agent,json=userAgent,proto3" json:"user_agent,omitempty"`
	BrowserName         string                     `protobuf:"bytes,5,opt,name=browser_name,json=browserName,proto3" json:"browser_name,omitempty"`
	BrowserMajorVersion int32                      `protobuf:"varint,6,opt,name=browser_major_version,json=browserMajorVersion,proto3" json:"browser_major_version,omitempty"`
	Disposition         SecurityReport_Disposition `protobuf:"varint,7,opt,name=disposition,proto3,enum=securityreport.SecurityReport_Disposition" json:"disposition,omitempty"`
	// this field will hold an extension of the base SecurityReport,
	// only one extension can be set for any given request
	//
	// Types that are assignable to ReportExtension:
	//
	//	*SecurityReport_CspReport
	//	*SecurityReport_DeprecationReport
	ReportExtension isSecurityReport_ReportExtension `protobuf_oneof:"ReportExtension"`
}

type isSecurityReport_ReportExtension interface {
	isSecurityReport_ReportExtension()
}

type SecurityReport_CspReport struct {
	CspReport *CspReport `protobuf:"bytes,8,opt,name=csp_report,json=cspReport,proto3,oneof"`
}

type SecurityReport_DeprecationReport struct {
	DeprecationReport *DeprecationReport `protobuf:"bytes,9,opt,name=deprecation_report,json=deprecationReport,proto3,oneof"`
}

func (*SecurityReport_CspReport) isSecurityReport_ReportExtension() {}

func (*SecurityReport_DeprecationReport) isSecurityReport_ReportExtension() {}

func ParseReport(data []byte) ([]*SecurityReport, error) {
	var buf []map[string]interface{}
	if err := json.Unmarshal(data, &buf); err != nil {
		return nil, err
	}

	var reports []*SecurityReport
	for _, b := range buf {
		r, err := mapToSecurityReport(b)
		if err != nil {
			return nil, err
		}
		reports = append(reports, r)
	}

	return reports, nil
}

func (r *SecurityReport) Validate() error {
	if r.ReportChecksum == "" {
		return fmt.Errorf("report_checksum is required")
	}

	if r.ReportTime == 0 {
		return fmt.Errorf("report_time is required")
	}

	if r.ReportCount == 0 {
		return fmt.Errorf("report_count is required")
	}

	if r.UserAgent == "" {
		return fmt.Errorf("user_agent is required")
	}

	if r.BrowserName == "" {
		return fmt.Errorf("browser_name is required")
	}

	if r.BrowserMajorVersion == 0 {
		return fmt.Errorf("browser_major_version is required")
	}

	if r.Disposition == 0 {
		return fmt.Errorf("disposition is required")
	}

	return nil
}

func mapToSHA256HexString(m map[string]interface{}) (string, error) {
	deserialized, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	checksum := sha256.Sum256(deserialized)
	return hex.EncodeToString(checksum[:]), nil
}

func mapToSecurityReport(m map[string]interface{}) (*SecurityReport, error) {
	sr := &SecurityReport{}
	now := time.Now().UnixMilli()
	checksum, err := mapToSHA256HexString(m)
	if err != nil {
		return nil, err
	}
	sr.ReportChecksum = checksum
	sr.ReportCount = int64(1)
	sr.Disposition = SecurityReport_DISPOSITION_UNKNOWN

	// the report has "age" field that is the offset from the report's timestamp.
	// https://w3c.github.io/reporting/#serialize-reports
	//
	// NOTE: currently the report doesn't have "timestamp" field, so use server side
	// current time.
	if age, ok := m["age"].(float64); ok {
		sr.ReportTime = now - int64(age)
	}
	if ua, ok := m["user_agent"].(string); ok {
		sr.UserAgent = ua
	}
	var typ string
	var body map[string]interface{}
	var ok bool
	if typ, ok = m["type"].(string); !ok {
		return nil, fmt.Errorf("unexpected report type: %v", m)
	}
	if body, ok = m["body"].(map[string]interface{}); !ok {
		return nil, fmt.Errorf("unexpected report type: %v", m)
	}
	switch typ {
	case "csp-violation":
		csp := &CspReport{}
		if duri, ok := body["documentURL"].(string); ok {
			csp.DocumentUri = duri
		} else {
			log.Warnf("unexpected documentURL: %#v", body["documentURL"])
		}
		if ref, ok := body["referrer"].(string); ok {
			csp.Referrer = ref
		} else {
			log.Warnf("unexpected referrer: %#v", body["referrer"])
		}
		if buri, ok := body["blockedURL"].(string); ok {
			csp.BlockedUri = buri
		} else {
			log.Warnf("unexpected blockedURL: %#v", body["blockedURL"])
		}
		if vd, ok := body["violatedDirective"].(string); ok {
			csp.ViolatedDirective = vd
		} else {
			log.Warnf("unexpected violatedDirective: %#v", body["violatedDirective"])
		}
		if ed, ok := body["effectiveDirective"].(string); ok {
			csp.EffectiveDirective = ed
		} else {
			log.Warnf("unexpected effectiveDirective: %#v", body["effectiveDirective"])
		}
		if sf, ok := body["sourceFile"].(string); ok {
			csp.SourceFile = sf
		} else {
			log.Warnf("unexpected sourceFile: %#v", body["sourceFile"])
		}
		if ln, ok := body["lineNumber"].(float64); ok {
			csp.LineNumber = int32(ln)
		} else {
			log.Warnf("unexpected lineNumber: %#v", body["lineNumber"])
		}
		if cn, ok := body["columnNumber"].(float64); ok {
			csp.ColumnNumber = int32(cn)
		} else {
			log.Warnf("unexpected columnNumber: %#v", body["columnNumber"])
		}
		if ss, ok := body["scriptSample"].(string); ok {
			csp.ScriptSample = ss
		} else {
			log.Warnf("unexpected scriptSample: %#v", body["scriptSample"])
		}
		sr.ReportExtension = &SecurityReport_CspReport{CspReport: csp}

		switch body["disposition"].(string) {
		case "enforce":
			sr.Disposition = SecurityReport_ENFORCED
		case "report":
			sr.Disposition = SecurityReport_REPORTING
		default:
		}
	case "deprecation":
		dep := &DeprecationReport{}
		if id, ok := body["id"].(string); ok {
			dep.Id = id
		} else {
			log.Warnf("unexpected id: %#v", body["id"])
		}
		if ar, ok := body["anticipatedRemoval"].(string); ok {
			dep.AnticipatedRemoval = ar
		} else {
			log.Warnf("unexpected anticipatedRemoval: %#v", body["anticipatedRemoval"])
		}
		if ln, ok := body["lineNumber"].(float64); ok {
			dep.LineNumber = int32(ln)
		} else {
			log.Warnf("unexpected lineNumber: %#v", body["lineNumber"])
		}
		if cn, ok := body["columnNumber"].(float64); ok {
			dep.ColumnNumber = int32(cn)
		} else {
			log.Warnf("unexpected columnNumber: %#v", body["columnNumber"])
		}
		if m, ok := body["message"].(string); ok {
			dep.Message = m
		} else {
			log.Warnf("unexpected message: %#v", body["message"])
		}
		if sf, ok := body["sourceFile"].(string); ok {
			dep.SourceFile = sf
		} else {
			log.Warnf("unexpected sourceFile: %#v", body["sourceFile"])
		}
		sr.ReportExtension = &SecurityReport_DeprecationReport{DeprecationReport: dep}
	}

	return sr, nil
}

func WriteReportsToBigQuery(ctx context.Context, project, dataset, table string, reports []*SecurityReport) error {
	if len(reports) == 0 {
		return nil
	}
	bq, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return fmt.Errorf("connecting to bq: %w", err)
	}

	ins := bq.Dataset(dataset).Table(table).Inserter()
	if err := ins.Put(ctx, reports); err != nil {
		return fmt.Errorf("uploading to bq: %w", err)
	}
	return nil
}
