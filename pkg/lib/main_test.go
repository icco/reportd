package lib

import (
	"strings"
	"testing"
)

func TestValidateService(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		wantErr bool
	}{
		{
			name:    "valid service",
			arg:     "reportd",
			wantErr: false,
		},
		{
			name:    "valid service 2",
			arg:     "reportd23",
			wantErr: false,
		},
		{
			name:    "invalid service",
			arg:     "reportd ",
			wantErr: true,
		},
		{
			name:    "newline",
			arg:     "reportd\n",
			wantErr: true,
		},
		{
			name:    "empty",
			arg:     "",
			wantErr: true,
		},
		// Edge cases for fuzz resistance
		{
			name:    "valid with hyphens and underscores",
			arg:     "my-service_v2",
			wantErr: false,
		},
		{
			name:    "valid single char",
			arg:     "a",
			wantErr: false,
		},
		{
			name:    "valid max length 32",
			arg:     strings.Repeat("a", 32),
			wantErr: false,
		},
		{
			name:    "exceeds max length 33",
			arg:     strings.Repeat("a", 33),
			wantErr: true,
		},
		{
			name:    "way over max length",
			arg:     strings.Repeat("x", 10000),
			wantErr: true,
		},
		{
			name:    "null byte",
			arg:     "service\x00name",
			wantErr: true,
		},
		{
			name:    "only null byte",
			arg:     "\x00",
			wantErr: true,
		},
		{
			name:    "tab character",
			arg:     "service\tname",
			wantErr: true,
		},
		{
			name:    "carriage return",
			arg:     "service\rname",
			wantErr: true,
		},
		{
			name:    "CRLF injection",
			arg:     "svc\r\nX-Injected: true",
			wantErr: true,
		},
		{
			name:    "unicode emoji",
			arg:     "service😀",
			wantErr: true,
		},
		{
			name:    "unicode homoglyph",
			arg:     "ѕervice", // Cyrillic 'ѕ'
			wantErr: true,
		},
		{
			name:    "SQL injection attempt",
			arg:     "'; DROP TABLE reports;--",
			wantErr: true,
		},
		{
			name:    "path traversal",
			arg:     "../../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "path traversal backslash",
			arg:     `..\..\windows`,
			wantErr: true,
		},
		{
			name:    "URL-encoded slash",
			arg:     "service%2F..%2Fetc",
			wantErr: true,
		},
		{
			name:    "XSS attempt",
			arg:     "<script>alert(1)</script>",
			wantErr: true,
		},
		{
			name:    "spaces only",
			arg:     "   ",
			wantErr: true,
		},
		{
			name:    "dots",
			arg:     "my.service",
			wantErr: true,
		},
		{
			name:    "colon",
			arg:     "svc:8080",
			wantErr: true,
		},
		{
			name:    "at sign",
			arg:     "user@host",
			wantErr: true,
		},
		{
			name:    "pipe",
			arg:     "svc|cat /etc/passwd",
			wantErr: true,
		},
		{
			name:    "backtick command injection",
			arg:     "svc`id`",
			wantErr: true,
		},
		{
			name:    "dollar command substitution",
			arg:     "svc$(id)",
			wantErr: true,
		},
		{
			name:    "curly braces",
			arg:     "svc{test}",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateService(tt.arg); (err != nil) != tt.wantErr {
				t.Errorf("ValidateService(%q) error = %v, wantErr %v", tt.arg, err, tt.wantErr)
			}
		})
	}
}
