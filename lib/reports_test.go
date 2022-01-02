package lib

import (
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"
	"time"
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
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			data, err := ParseReport(tc.ContentType, tc.JSON)
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

	files, err := ioutil.ReadDir("./reports-examples")
	if err != nil {
		t.Error(err)
	}

	for _, file := range files {
		json, err := ioutil.ReadFile(filepath.Join(".", "reports-examples", file.Name()))
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
			data, err := ParseReport(tc.ContentType, tc.JSON)
			if err != nil {
				t.Error(err)
			}

			if data == nil {
				t.Error("data should not be nil")
			}
		})
	}
}
