package reporting

import (
	"os"
	"path/filepath"
	"testing"
)

type reportTest struct {
	Name        string
	ContentType string
	JSON        string
	Expect      *SecurityReport
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
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			data, err := ParseReport(tc.JSON)
			if err != nil {
				t.Error(err)
			}

			if data == nil {
				t.Error("data should not be nil")
			}
		})
	}
}
