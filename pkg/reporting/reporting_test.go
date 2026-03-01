package reporting

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseReportAllExamples(t *testing.T) {
	files, err := os.ReadDir("./examples")
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		file := file
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
	if data.Deprecation.Body.Id != "websql" {
		t.Errorf("expected id 'websql', got %q", data.Deprecation.Body.Id)
	}
	if data.Deprecation.Body.Message != "WebSQL is deprecated" {
		t.Errorf("expected message 'WebSQL is deprecated', got %q", data.Deprecation.Body.Message)
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
