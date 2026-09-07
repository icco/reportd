package reporting

import (
	"fmt"
	"testing"
)

// TestParseReportRejectsMalformedBodyPerType covers the type-specific
// json.Unmarshal error return in each switch arm. Every report type declares
// "body" as a struct, so handing it an array fails to unmarshal while the
// leading type sniff still succeeds — which is the only way to reach the arm
// and then fail inside it.
func TestParseReportRejectsMalformedBodyPerType(t *testing.T) {
	reportTypes := []string{
		"csp-violation",
		"deprecation",
		"permissions-policy-violation",
		"intervention",
		"crash",
		"coep",
		"coop",
		"document-policy-violation",
	}

	for _, rt := range reportTypes {
		t.Run(rt, func(t *testing.T) {
			body := fmt.Sprintf(`{"type":%q,"body":[]}`, rt)

			got, err := ParseReport(body, "svc")
			if err == nil {
				t.Fatalf("ParseReport(%s) error = nil, want an unmarshal error (got %+v)", rt, got)
			}
			if got != nil {
				t.Errorf("ParseReport() = %+v, want nil on error", got)
			}
		})
	}
}

func TestParseReportRejectsInvalidJSON(t *testing.T) {
	if _, err := ParseReport("{not json", "svc"); err == nil {
		t.Fatal("ParseReport() error = nil, want an error")
	}
}

// TestParseReportKeepsUnknownTypeInRawJSON covers the default switch arm.
func TestParseReportKeepsUnknownTypeInRawJSON(t *testing.T) {
	const body = `{"type":"brand-new-report-type","body":{"anything":1}}`

	got, err := ParseReport(body, "svc")
	if err != nil {
		t.Fatalf("ParseReport() error = %v", err)
	}
	if got.RawJSON != body {
		t.Errorf("RawJSON = %q, want the original payload", got.RawJSON)
	}
	if got.ReportType.StringVal != "brand-new-report-type" {
		t.Errorf("ReportType = %q, want the unknown type preserved", got.ReportType.StringVal)
	}
}
