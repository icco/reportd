package reporting

import (
	"encoding/json"
	"fmt"

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

func ParseReport(data []byte) (*SecurityReport, error) {
	var r SecurityReport
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}

	return &r, nil
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
