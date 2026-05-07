package reporting

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetReportSchema(t *testing.T) {
	schema, err := getReportSchema()
	if err != nil {
		t.Fatalf("getReportSchema() error = %v", err)
	}
	if len(schema) == 0 {
		t.Error("inferred schema should not be empty")
	}
}

func TestParseReportAllExamples(t *testing.T) {
	files, err := os.ReadDir("./examples")
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {

		t.Run(file.Name(), func(t *testing.T) {
			t.Parallel()
			body, err := os.ReadFile(filepath.Join(".", "examples", file.Name()))
			if err != nil {
				t.Fatal(err)
			}

			data, err := ParseReport(string(body), "test")
			if err != nil {
				t.Fatalf("ParseReport returned error: %v", err)
			}

			if data == nil {
				t.Fatal("ParseReport returned nil")
			}

			if data.Service.String() != "test" {
				t.Errorf("expected service 'test', got %q", data.Service.StringVal)
			}

			if !data.ReportType.Valid || data.ReportType.StringVal == "" {
				t.Error("ReportType should not be empty")
			}

			if data.RawJSON == "" {
				t.Error("RawJSON should not be empty")
			}
		})
	}
}

func TestParseCSPViolation(t *testing.T) {
	body := `{
		"type": "csp-violation",
		"url": "https://example.com/",
		"body": {
			"document_uri": "https://example.com/",
			"blocked_uri": "https://evil.com/script.js",
			"effective_directive": "script-src-elem",
			"original_policy": "default-src 'self'",
			"source_file": "https://example.com/app.js",
			"line_number": 10,
			"column_number": 5
		}
	}`

	data, err := ParseReport(body, "mysite")
	if err != nil {
		t.Fatal(err)
	}

	if data.ReportType.StringVal != "csp-violation" {
		t.Errorf("expected type 'csp-violation', got %q", data.ReportType.StringVal)
	}
	if data.CSP == nil {
		t.Fatal("CSP should not be nil")
	}
	if data.CSP.URL != "https://example.com/" {
		t.Errorf("expected URL 'https://example.com/', got %q", data.CSP.URL)
	}
	if data.CSP.Body.BlockedUri != "https://evil.com/script.js" {
		t.Errorf("expected blocked_uri 'https://evil.com/script.js', got %q", data.CSP.Body.BlockedUri)
	}
	if data.CSP.Body.EffectiveDirective != "script-src-elem" {
		t.Errorf("expected effective_directive 'script-src-elem', got %q", data.CSP.Body.EffectiveDirective)
	}
	if data.CSP.Body.LineNumber != 10 {
		t.Errorf("expected line_number 10, got %d", data.CSP.Body.LineNumber)
	}
}

func TestParseDeprecation(t *testing.T) {
	body := `{
		"type": "deprecation",
		"url": "https://example.com/",
		"body": {
			"id": "websql",
			"message": "WebSQL is deprecated",
			"source_file": "https://example.com/db.js",
			"line_number": 42,
			"column_number": 8
		}
	}`

	data, err := ParseReport(body, "mysite")
	if err != nil {
		t.Fatal(err)
	}

	if data.ReportType.StringVal != "deprecation" {
		t.Errorf("expected type 'deprecation', got %q", data.ReportType.StringVal)
	}
	if data.Deprecation == nil {
		t.Fatal("Deprecation should not be nil")
	}
	if data.Deprecation.Body.Id.StringVal != "websql" {
		t.Errorf("expected id 'websql', got %q", data.Deprecation.Body.Id.StringVal)
	}
	if data.Deprecation.Body.Message.StringVal != "WebSQL is deprecated" {
		t.Errorf("expected message 'WebSQL is deprecated', got %q", data.Deprecation.Body.Message.StringVal)
	}
}

func TestParsePermissionsPolicy(t *testing.T) {
	body := `{
		"type": "permissions-policy-violation",
		"url": "https://example.com/page",
		"body": {
			"featureId": "camera",
			"sourceFile": "https://example.com/app.js",
			"lineNumber": 15,
			"columnNumber": 3,
			"disposition": "enforce",
			"message": "camera is not allowed"
		}
	}`

	data, err := ParseReport(body, "mysite")
	if err != nil {
		t.Fatal(err)
	}

	if data.ReportType.StringVal != "permissions-policy-violation" {
		t.Errorf("expected type 'permissions-policy-violation', got %q", data.ReportType.StringVal)
	}
	if data.PermissionsPolicy == nil {
		t.Fatal("PermissionsPolicy should not be nil")
	}
	if data.PermissionsPolicy.Body.FeatureId != "camera" {
		t.Errorf("expected featureId 'camera', got %q", data.PermissionsPolicy.Body.FeatureId)
	}
	if data.PermissionsPolicy.Body.Disposition != "enforce" {
		t.Errorf("expected disposition 'enforce', got %q", data.PermissionsPolicy.Body.Disposition)
	}
	if data.PermissionsPolicy.Body.LineNumber != 15 {
		t.Errorf("expected lineNumber 15, got %d", data.PermissionsPolicy.Body.LineNumber)
	}
}

func TestParseIntervention(t *testing.T) {
	body := `{
		"type": "intervention",
		"url": "https://example.com/page",
		"body": {
			"id": "HeavyAdIntervention",
			"message": "Ad removed for CPU usage",
			"sourceFile": "https://example.com/ads.js",
			"lineNumber": 50,
			"columnNumber": 1
		}
	}`

	data, err := ParseReport(body, "mysite")
	if err != nil {
		t.Fatal(err)
	}

	if data.ReportType.StringVal != "intervention" {
		t.Errorf("expected type 'intervention', got %q", data.ReportType.StringVal)
	}
	if data.Intervention == nil {
		t.Fatal("Intervention should not be nil")
	}
	if data.Intervention.Body.Id != "HeavyAdIntervention" {
		t.Errorf("expected id 'HeavyAdIntervention', got %q", data.Intervention.Body.Id)
	}
	if data.Intervention.Body.Message != "Ad removed for CPU usage" {
		t.Errorf("expected message 'Ad removed for CPU usage', got %q", data.Intervention.Body.Message)
	}
}

func TestParseCrash(t *testing.T) {
	body := `{
		"type": "crash",
		"url": "https://example.com/page",
		"body": {
			"reason": "unresponsive"
		}
	}`

	data, err := ParseReport(body, "mysite")
	if err != nil {
		t.Fatal(err)
	}

	if data.ReportType.StringVal != "crash" {
		t.Errorf("expected type 'crash', got %q", data.ReportType.StringVal)
	}
	if data.Crash == nil {
		t.Fatal("Crash should not be nil")
	}
	if data.Crash.Body.Reason != "unresponsive" {
		t.Errorf("expected reason 'unresponsive', got %q", data.Crash.Body.Reason)
	}
}

func TestParseCOEP(t *testing.T) {
	body := `{
		"type": "coep",
		"url": "https://example.com/page",
		"body": {
			"type": "corp",
			"blockedURL": "https://cdn.example.com/img.png",
			"destination": "image",
			"disposition": "enforce"
		}
	}`

	data, err := ParseReport(body, "mysite")
	if err != nil {
		t.Fatal(err)
	}

	if data.ReportType.StringVal != "coep" {
		t.Errorf("expected type 'coep', got %q", data.ReportType.StringVal)
	}
	if data.COEP == nil {
		t.Fatal("COEP should not be nil")
	}
	if data.COEP.Body.BlockedURL != "https://cdn.example.com/img.png" {
		t.Errorf("expected blockedURL 'https://cdn.example.com/img.png', got %q", data.COEP.Body.BlockedURL)
	}
	if data.COEP.Body.Disposition != "enforce" {
		t.Errorf("expected disposition 'enforce', got %q", data.COEP.Body.Disposition)
	}
}

func TestParseCOOP(t *testing.T) {
	body := `{
		"type": "coop",
		"url": "https://example.com/page",
		"body": {
			"type": "navigation-to-response",
			"property": "opener",
			"effectivePolicy": "same-origin",
			"nextResponseURL": "https://other.example.com/",
			"previousResponseURL": "https://example.com/page"
		}
	}`

	data, err := ParseReport(body, "mysite")
	if err != nil {
		t.Fatal(err)
	}

	if data.ReportType.StringVal != "coop" {
		t.Errorf("expected type 'coop', got %q", data.ReportType.StringVal)
	}
	if data.COOP == nil {
		t.Fatal("COOP should not be nil")
	}
	if data.COOP.Body.EffectivePolicy != "same-origin" {
		t.Errorf("expected effectivePolicy 'same-origin', got %q", data.COOP.Body.EffectivePolicy)
	}
	if data.COOP.Body.Property != "opener" {
		t.Errorf("expected property 'opener', got %q", data.COOP.Body.Property)
	}
}

func TestParseDocumentPolicy(t *testing.T) {
	body := `{
		"type": "document-policy-violation",
		"url": "https://example.com/page",
		"body": {
			"featureId": "oversized-images",
			"sourceFile": "https://example.com/index.html",
			"lineNumber": 15,
			"columnNumber": 1,
			"disposition": "enforce",
			"message": "Image exceeds size limit"
		}
	}`

	data, err := ParseReport(body, "mysite")
	if err != nil {
		t.Fatal(err)
	}

	if data.ReportType.StringVal != "document-policy-violation" {
		t.Errorf("expected type 'document-policy-violation', got %q", data.ReportType.StringVal)
	}
	if data.DocumentPolicy == nil {
		t.Fatal("DocumentPolicy should not be nil")
	}
	if data.DocumentPolicy.Body.FeatureId != "oversized-images" {
		t.Errorf("expected featureId 'oversized-images', got %q", data.DocumentPolicy.Body.FeatureId)
	}
	if data.DocumentPolicy.Body.Message != "Image exceeds size limit" {
		t.Errorf("expected message 'Image exceeds size limit', got %q", data.DocumentPolicy.Body.Message)
	}
}

func TestParseUnknownType(t *testing.T) {
	body := `{
		"type": "some-future-type",
		"url": "https://example.com/page",
		"body": { "foo": "bar" }
	}`

	data, err := ParseReport(body, "mysite")
	if err != nil {
		t.Fatal(err)
	}

	if data.ReportType.StringVal != "some-future-type" {
		t.Errorf("expected type 'some-future-type', got %q", data.ReportType.StringVal)
	}
	if data.RawJSON == "" {
		t.Error("RawJSON should be preserved for unknown types")
	}
	if data.CSP != nil || data.Deprecation != nil || data.Crash != nil {
		t.Error("known type fields should be nil for unknown types")
	}
}

func TestParseReportEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		service string
		wantErr bool
	}{
		// Invalid JSON
		{
			name:    "empty body",
			body:    "",
			service: "test",
			wantErr: true,
		},
		{
			name:    "null JSON",
			body:    "null",
			service: "test",
			wantErr: false, // json.Unmarshal(null, &struct{}) succeeds with zero values
		},
		{
			name:    "JSON array",
			body:    `[{"type":"csp-violation"}]`,
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
			body:    `{"type":"csp-vio`,
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
			name:    "invalid JSON with trailing comma",
			body:    `{"type":"crash",}`,
			service: "test",
			wantErr: true,
		},
		// Missing type field
		{
			name:    "missing type field",
			body:    `{"url":"https://example.com/","body":{}}`,
			service: "test",
			wantErr: false, // falls through to default case, stores raw JSON
		},
		{
			name:    "empty type field",
			body:    `{"type":"","url":"https://example.com/","body":{}}`,
			service: "test",
			wantErr: false, // unknown type, stores raw JSON
		},
		// Empty object
		{
			name:    "empty JSON object",
			body:    `{}`,
			service: "test",
			wantErr: false, // type is "", falls through to default
		},
		// Type field as wrong JSON type
		{
			name:    "type field as number",
			body:    `{"type":42,"url":"https://example.com/"}`,
			service: "test",
			wantErr: true, // can't unmarshal number into string
		},
		{
			name:    "type field as array",
			body:    `{"type":["csp-violation"],"url":"https://example.com/"}`,
			service: "test",
			wantErr: true,
		},
		{
			name:    "type field as null",
			body:    `{"type":null,"url":"https://example.com/"}`,
			service: "test",
			wantErr: false, // null unmarshals to zero value ""
		},
		// Large payloads
		{
			name:    "very long URL",
			body:    `{"type":"csp-violation","url":"https://example.com/` + strings.Repeat("a", 100000) + `","body":{}}`,
			service: "test",
			wantErr: false,
		},
		{
			name:    "very long body field",
			body:    `{"type":"crash","url":"https://example.com/","body":{"reason":"` + strings.Repeat("x", 100000) + `"}}`,
			service: "test",
			wantErr: false,
		},
		// XSS/injection in fields
		{
			name:    "XSS in URL",
			body:    `{"type":"csp-violation","url":"javascript:alert(document.cookie)","body":{"blocked_uri":"<script>alert(1)</script>"}}`,
			service: "test",
			wantErr: false,
		},
		{
			name:    "SQL injection in body fields",
			body:    `{"type":"csp-violation","url":"https://example.com/","body":{"document_uri":"'; DROP TABLE reports;--","blocked_uri":"1' OR '1'='1"}}`,
			service: "test",
			wantErr: false,
		},
		{
			name:    "null bytes in strings",
			body:    `{"type":"csp-violation","url":"https://example.com/\u0000","body":{"document_uri":"test\u0000inject"}}`,
			service: "test",
			wantErr: false,
		},
		// Extra/unknown fields (should be silently ignored)
		{
			name:    "extra fields in root",
			body:    `{"type":"crash","url":"https://example.com/","body":{"reason":"oom"},"__proto__":{"admin":true},"constructor":{"prototype":{"isAdmin":true}}}`,
			service: "test",
			wantErr: false,
		},
		// Deeply nested JSON
		{
			name:    "deeply nested body",
			body:    `{"type":"crash","url":"https://example.com/","body":{"reason":"oom","nested":` + strings.Repeat(`{"a":`, 100) + `1` + strings.Repeat(`}`, 100) + `}}`,
			service: "test",
			wantErr: false,
		},
		// All valid report types with minimal body
		{
			name:    "minimal csp-violation",
			body:    `{"type":"csp-violation","url":"","body":{}}`,
			service: "test",
			wantErr: false,
		},
		{
			name:    "minimal deprecation",
			body:    `{"type":"deprecation","url":"","body":{}}`,
			service: "test",
			wantErr: false,
		},
		{
			name:    "minimal permissions-policy-violation",
			body:    `{"type":"permissions-policy-violation","url":"","body":{}}`,
			service: "test",
			wantErr: false,
		},
		{
			name:    "minimal intervention",
			body:    `{"type":"intervention","url":"","body":{}}`,
			service: "test",
			wantErr: false,
		},
		{
			name:    "minimal crash",
			body:    `{"type":"crash","url":"","body":{}}`,
			service: "test",
			wantErr: false,
		},
		{
			name:    "minimal coep",
			body:    `{"type":"coep","url":"","body":{}}`,
			service: "test",
			wantErr: false,
		},
		{
			name:    "minimal coop",
			body:    `{"type":"coop","url":"","body":{}}`,
			service: "test",
			wantErr: false,
		},
		{
			name:    "minimal document-policy-violation",
			body:    `{"type":"document-policy-violation","url":"","body":{}}`,
			service: "test",
			wantErr: false,
		},
		// Case sensitivity
		{
			name:    "uppercase type",
			body:    `{"type":"CSP-VIOLATION","url":"https://example.com/","body":{}}`,
			service: "test",
			wantErr: false, // treated as unknown type
		},
		{
			name:    "mixed case type",
			body:    `{"type":"Crash","url":"https://example.com/","body":{}}`,
			service: "test",
			wantErr: false, // treated as unknown type
		},
	}

	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data, err := ParseReport(tc.body, tc.service)
			if (err != nil) != tc.wantErr {
				t.Errorf("ParseReport() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if err == nil {
				if data == nil {
					t.Error("expected non-nil data when no error")
					return
				}
				if data.RawJSON != tc.body {
					t.Errorf("RawJSON should preserve original body")
				}
				if data.Service.StringVal != tc.service {
					t.Errorf("expected service %q, got %q", tc.service, data.Service.StringVal)
				}
				if !data.Time.Valid {
					t.Error("time should always be set")
				}
			}
		})
	}
}

func TestParseReportCaseSensitiveTypes(t *testing.T) {
	// Verify that type matching is case-sensitive (uppercase should NOT match known types)
	body := `{"type":"CSP-VIOLATION","url":"https://example.com/","body":{"blocked_uri":"https://evil.com/"}}`
	data, err := ParseReport(body, "test")
	if err != nil {
		t.Fatal(err)
	}
	// Should be treated as unknown — CSP field should remain nil
	if data.CSP != nil {
		t.Error("uppercase CSP-VIOLATION should not match csp-violation handler")
	}
}

func TestParseReportPreservesRawJSON(t *testing.T) {
	body := `{"type":"csp-violation","url":"https://example.com/","body":{"document_uri":"https://example.com/"}}`
	data, err := ParseReport(body, "test")
	if err != nil {
		t.Fatal(err)
	}
	if data.RawJSON != body {
		t.Error("RawJSON should exactly match the input body")
	}
}

func TestParseReportDeprecationNullStringFields(t *testing.T) {
	// Test that deprecation NullString fields are properly populated
	body := `{"type":"deprecation","url":"https://example.com/","body":{"id":"websql","anticipated_removal":"2025-01-01","message":"deprecated"}}`
	data, err := ParseReport(body, "test")
	if err != nil {
		t.Fatal(err)
	}
	if !data.Deprecation.Body.Id.Valid || data.Deprecation.Body.Id.StringVal != "websql" {
		t.Errorf("expected id 'websql', got %+v", data.Deprecation.Body.Id)
	}
	if !data.Deprecation.Body.AnticipatedRemoval.Valid || data.Deprecation.Body.AnticipatedRemoval.StringVal != "2025-01-01" {
		t.Errorf("expected anticipated_removal '2025-01-01', got %+v", data.Deprecation.Body.AnticipatedRemoval)
	}
	if !data.Deprecation.Body.Message.Valid || data.Deprecation.Body.Message.StringVal != "deprecated" {
		t.Errorf("expected message 'deprecated', got %+v", data.Deprecation.Body.Message)
	}
}

func TestParseReportDeprecationEmptyNullFields(t *testing.T) {
	// When deprecation fields are missing, NullString should be invalid
	body := `{"type":"deprecation","url":"https://example.com/","body":{}}`
	data, err := ParseReport(body, "test")
	if err != nil {
		t.Fatal(err)
	}
	if data.Deprecation.Body.Id.Valid {
		t.Error("id should not be valid when missing")
	}
	if data.Deprecation.Body.AnticipatedRemoval.Valid {
		t.Error("anticipated_removal should not be valid when missing")
	}
	if data.Deprecation.Body.Message.Valid {
		t.Error("message should not be valid when missing")
	}
}
