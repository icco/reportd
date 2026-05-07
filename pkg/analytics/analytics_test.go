package analytics

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cloud.google.com/go/bigquery"
)

func TestGetAnalyticsSchema(t *testing.T) {
	_, err := getAnalyticsSchema()
	if err != nil {
		t.Error(err)
	}
}

type analyticsTest struct {
	Name string
	JSON string
}

func TestParseAnalyticsParsesWebVitals(t *testing.T) {
	var tests []analyticsTest

	files, err := os.ReadDir("./examples")
	if err != nil {
		t.Error(err)
	}

	for _, file := range files {
		json, err := os.ReadFile(filepath.Join(".", "examples", file.Name()))
		if err != nil {
			t.Error(err)
		}

		tests = append(tests, analyticsTest{
			Name: file.Name(),
			JSON: string(json),
		})
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			data, err := ParseAnalytics(tc.JSON, "test")
			if err != nil {
				t.Error(err)
			}

			if data == nil {
				t.Error("data should not be nil")
			}
		})
	}
}

func TestParseAnalyticsEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		service string
		wantErr bool
	}{
		{
			name:    "empty body",
			body:    "",
			service: "test",
			wantErr: true,
		},
		{
			name:    "empty JSON object",
			body:    "{}",
			service: "test",
			wantErr: false,
		},
		{
			name:    "null JSON",
			body:    "null",
			service: "test",
			wantErr: false, // json.Unmarshal(null, &struct{}) is a no-op
		},
		{
			name:    "JSON array instead of object",
			body:    `[{"name":"LCP","value":100}]`,
			service: "test",
			wantErr: true,
		},
		{
			name:    "just a string",
			body:    `"hello"`,
			service: "test",
			wantErr: true,
		},
		{
			name:    "just a number",
			body:    `42`,
			service: "test",
			wantErr: true,
		},
		{
			name:    "truncated JSON",
			body:    `{"name":"LCP","val`,
			service: "test",
			wantErr: true,
		},
		{
			name:    "invalid JSON with trailing comma",
			body:    `{"name":"LCP",}`,
			service: "test",
			wantErr: true,
		},
		{
			name:    "binary garbage",
			body:    "\x00\x01\x02\xff\xfe",
			service: "test",
			wantErr: true,
		},
		{
			name:    "negative value",
			body:    `{"name":"CLS","value":-1.5,"delta":-0.5,"id":"v1-neg"}`,
			service: "test",
			wantErr: false,
		},
		{
			name:    "zero values",
			body:    `{"name":"CLS","value":0,"delta":0,"id":"v1-zero"}`,
			service: "test",
			wantErr: false,
		},
		{
			name:    "very large value",
			body:    `{"name":"LCP","value":999999999999.99,"delta":0,"id":"v1-big"}`,
			service: "test",
			wantErr: false,
		},
		{
			name:    "value as string type confusion",
			body:    `{"name":"LCP","value":"not_a_number","delta":0,"id":"v1-str"}`,
			service: "test",
			wantErr: true,
		},
		{
			name:    "extra unknown fields",
			body:    `{"name":"LCP","value":100,"delta":50,"id":"v1","unknown_field":"injected","__proto__":{"admin":true}}`,
			service: "test",
			wantErr: false,
		},
		{
			name:    "deeply nested JSON bomb",
			body:    `{"name":"LCP","value":1,"delta":1,"id":"v1","label":"` + strings.Repeat(`a`, 10000) + `"}`,
			service: "test",
			wantErr: false,
		},
		{
			name:    "HTML in string field",
			body:    `{"name":"<script>alert(1)</script>","value":100,"delta":50,"id":"v1-xss"}`,
			service: "test",
			wantErr: false,
		},
		{
			name:    "unicode in fields",
			body:    `{"name":"LCP","value":100,"delta":50,"id":"v1-\u0000null"}`,
			service: "test",
			wantErr: false,
		},
		{
			name:    "empty service",
			body:    `{"name":"LCP","value":100,"delta":50,"id":"v1"}`,
			service: "",
			wantErr: false, // ParseAnalytics doesn't validate service
		},
		{
			name:    "duplicate keys in JSON",
			body:    `{"name":"LCP","value":100,"value":200,"delta":50,"id":"v1"}`,
			service: "test",
			wantErr: false, // Go json.Unmarshal takes last value
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data, err := ParseAnalytics(tc.body, tc.service)
			if (err != nil) != tc.wantErr {
				t.Errorf("ParseAnalytics() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if err == nil && data != nil {
				if !data.Service.Valid {
					t.Error("service should always be valid when no error")
				}
				if !data.Time.Valid {
					t.Error("time should always be valid when no error")
				}
			} else if err == nil {
				t.Fatal("expected non-nil data when no error")
			}
		})
	}
}

func TestParseAnalyticsDuplicateKeyTakesLast(t *testing.T) {
	body := `{"name":"LCP","value":100,"value":200,"delta":50,"id":"v1"}`
	data, err := ParseAnalytics(body, "test")
	if err != nil {
		t.Fatal(err)
	}
	if data.Value != 200 {
		t.Errorf("expected Go json.Unmarshal to take last duplicate key value 200, got %f", data.Value)
	}
}

func TestParseAnalyticsNaNAndInfinity(t *testing.T) {
	// JSON spec doesn't allow NaN/Inf, so these should fail to parse
	bodies := []string{
		`{"name":"LCP","value":NaN,"delta":0,"id":"v1"}`,
		`{"name":"LCP","value":Infinity,"delta":0,"id":"v1"}`,
		`{"name":"LCP","value":-Infinity,"delta":0,"id":"v1"}`,
	}
	for _, body := range bodies {
		_, err := ParseAnalytics(body, "test")
		if err == nil {
			t.Errorf("expected error for body %q", body)
		}
	}
}

func TestParseAnalyticsPreservesExtremeFloats(t *testing.T) {
	body := `{"name":"CLS","value":1.7976931348623157e+308,"delta":5e-324,"id":"v1-extreme"}`
	data, err := ParseAnalytics(body, "test")
	if err != nil {
		t.Fatal(err)
	}
	if data.Value != math.MaxFloat64 {
		t.Errorf("expected MaxFloat64, got %g", data.Value)
	}
	if data.Delta != math.SmallestNonzeroFloat64 {
		t.Errorf("expected SmallestNonzeroFloat64, got %g", data.Delta)
	}
}

func TestWriteAnalyticsToBigQueryValidation(t *testing.T) {
	// An invalid WebVital (Service unset) must be rejected before any
	// BigQuery client is constructed, so this test does not need network or
	// credentials.
	err := WriteAnalyticsToBigQuery(t.Context(), "p", "d", "tab", []*WebVital{{}})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "validating data") {
		t.Errorf("error %q should mention validation", err.Error())
	}
}

func TestWebVitalValidate(t *testing.T) {
	tests := []struct {
		name    string
		wv      *WebVital
		wantErr bool
	}{
		{
			name:    "valid",
			wv:      &WebVital{Service: bigquery.NullString{StringVal: "mysite", Valid: true}},
			wantErr: false,
		},
		{
			name:    "null service",
			wv:      &WebVital{},
			wantErr: true,
		},
		{
			name:    "empty service string",
			wv:      &WebVital{Service: bigquery.NullString{StringVal: "", Valid: true}},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.wv.Validate(); (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
