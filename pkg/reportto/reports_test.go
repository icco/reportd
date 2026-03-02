package reportto

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
)

func TestGetReportsSchema(t *testing.T) {
	_, err := getReportSchema()
	if err != nil {
		t.Error(err)
	}
}

type reportTest struct {
	Name        string
	ContentType string
	JSON        string
	Expect      *Report
}

func TestParseReport(t *testing.T) {
	tests := []reportTest{
		{
			Name:        "expect-ct-report",
			ContentType: "application/expect-ct-report+json",
			JSON:        `{"expect-ct-report":{"date-time":"2019-10-06T15:09:06.894Z","effective-expiration-date":"2019-10-06T15:09:06.894Z","hostname":"expect-ct-report.test","port":443,"scts":[],"served-certificate-chain":[],"validated-certificate-chain":[]}}`,
			Expect: &Report{
				ExpectCT: &ExpectCTReport{
					ExpectCTReport: ExpectCTSubReport{
						DateTime: time.Now(),
					},
				},
			},
		},
	}

	for _, tc := range tests {

		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			data, err := ParseReport(tc.ContentType, tc.JSON, "test")
			if err != nil {
				t.Error(err)
			}

			if data == nil {
				t.Error("data should not be nil")
			}

			if reflect.DeepEqual(data, tc.Expect) {
				t.Errorf("data is not accurate: %+v != %+v", data, tc.Expect)
			}
		})
	}
}

func TestParseReportParsesReportTo(t *testing.T) {
	var tests []reportTest

	files, err := os.ReadDir("./examples")
	if err != nil {
		t.Error(err)
	}

	for _, file := range files {
		json, err := os.ReadFile(filepath.Join(".", "examples", file.Name()))
		if err != nil {
			t.Error(err)
		}

		tests = append(tests, reportTest{
			Name:        file.Name(),
			ContentType: "application/reports+json",
			JSON:        string(json),
		})
	}

	for _, tc := range tests {

		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			data, err := ParseReport(tc.ContentType, tc.JSON, "test")
			if err != nil {
				t.Error(err)
			}

			if data == nil {
				t.Error("data should not be nil")
			}
		})
	}
}

func TestParseReportEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		service     string
		wantErr     bool
	}{
		// Invalid content types
		{
			name:        "empty content type",
			contentType: "",
			body:        "{}",
			service:     "test",
			wantErr:     true,
		},
		{
			name:        "text/plain content type",
			contentType: "text/plain",
			body:        "{}",
			service:     "test",
			wantErr:     true,
		},
		{
			name:        "application/json is not accepted",
			contentType: "application/json",
			body:        "{}",
			service:     "test",
			wantErr:     true,
		},
		{
			name:        "content type with charset param",
			contentType: "application/csp-report; charset=utf-8",
			body:        `{"csp-report":{"document-uri":"https://example.com/"}}`,
			service:     "test",
			wantErr:     false,
		},
		{
			name:        "content type with boundary",
			contentType: "application/csp-report; boundary=something",
			body:        `{"csp-report":{"document-uri":"https://example.com/"}}`,
			service:     "test",
			wantErr:     false,
		},
		// Invalid JSON bodies
		{
			name:        "empty body for csp-report",
			contentType: "application/csp-report",
			body:        "",
			service:     "test",
			wantErr:     true,
		},
		{
			name:        "empty body for reports+json",
			contentType: "application/reports+json",
			body:        "",
			service:     "test",
			wantErr:     true,
		},
		{
			name:        "empty body for expect-ct",
			contentType: "application/expect-ct-report+json",
			body:        "",
			service:     "test",
			wantErr:     true,
		},
		{
			name:        "null JSON for csp",
			contentType: "application/csp-report",
			body:        "null",
			service:     "test",
			wantErr:     false, // json.Unmarshal(null, &struct{}) is a no-op
		},
		{
			name:        "truncated JSON",
			contentType: "application/csp-report",
			body:        `{"csp-report":{"document-uri":"https:`,
			service:     "test",
			wantErr:     true,
		},
		{
			name:        "binary garbage",
			contentType: "application/csp-report",
			body:        "\x00\x01\x02\xff\xfe",
			service:     "test",
			wantErr:     true,
		},
		// Valid minimal payloads
		{
			name:        "empty CSP report object",
			contentType: "application/csp-report",
			body:        `{"csp-report":{}}`,
			service:     "test",
			wantErr:     false,
		},
		{
			name:        "empty reports array",
			contentType: "application/reports+json",
			body:        `[]`,
			service:     "test",
			wantErr:     false,
		},
		{
			name:        "empty expect-ct object",
			contentType: "application/expect-ct-report+json",
			body:        `{}`,
			service:     "test",
			wantErr:     false,
		},
		// Type confusion
		{
			name:        "reports+json gets object instead of array",
			contentType: "application/reports+json",
			body:        `{"type":"csp","url":"https://example.com/"}`,
			service:     "test",
			wantErr:     true,
		},
		{
			name:        "csp-report gets array instead of object",
			contentType: "application/csp-report",
			body:        `[{"csp-report":{}}]`,
			service:     "test",
			wantErr:     true,
		},
		// Large/adversarial payloads
		{
			name:        "very long string in field",
			contentType: "application/csp-report",
			body:        `{"csp-report":{"document-uri":"` + strings.Repeat("A", 100000) + `"}}`,
			service:     "test",
			wantErr:     false,
		},
		{
			name:        "many reports in array",
			contentType: "application/reports+json",
			body:        `[` + strings.Repeat(`{"type":"csp","url":"https://example.com/"},`, 999) + `{"type":"csp","url":"https://example.com/"}]`,
			service:     "test",
			wantErr:     false,
		},
		// XSS in fields
		{
			name:        "XSS in document-uri",
			contentType: "application/csp-report",
			body:        `{"csp-report":{"document-uri":"javascript:alert(1)","blocked-uri":"<img src=x onerror=alert(1)>"}}`,
			service:     "test",
			wantErr:     false,
		},
		// Empty service
		{
			name:        "empty service",
			contentType: "application/csp-report",
			body:        `{"csp-report":{}}`,
			service:     "",
			wantErr:     true, // Report.Validate() rejects empty service
		},
	}

	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data, err := ParseReport(tc.contentType, tc.body, tc.service)
			if (err != nil) != tc.wantErr {
				t.Errorf("ParseReport() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if err == nil && data == nil {
				t.Error("expected non-nil data when no error")
			}
		})
	}
}

func TestReportValidate(t *testing.T) {
	tests := []struct {
		name    string
		report  Report
		wantErr bool
	}{
		{
			name:    "valid service",
			report:  Report{Service: bqStr("mysite")},
			wantErr: false,
		},
		{
			name:    "null service",
			report:  Report{},
			wantErr: true,
		},
		{
			name:    "empty service string",
			report:  Report{Service: bqStr("")},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.report.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func bqStr(s string) bigquery.NullString {
	return bigquery.NullString{StringVal: s, Valid: true}
}
