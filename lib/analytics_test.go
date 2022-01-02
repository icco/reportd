package lib

import (
	"io/ioutil"
	"path/filepath"
	"testing"
)

type analyticsTest struct {
	Name string
	JSON string
}

func TestParseAnalyticsParsesWebVitals(t *testing.T) {
	var tests []analyticsTest

	files, err := ioutil.ReadDir("./analytics-examples")
	if err != nil {
		t.Error(err)
	}

	for _, file := range files {
		json, err := ioutil.ReadFile(filepath.Join(".", "analytics-examples", file.Name()))
		if err != nil {
			t.Error(err)
		}

		tests = append(tests, analyticsTest{
			Name: file.Name(),
			JSON: string(json),
		})
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			data, err := ParseAnalytics(tc.JSON)
			if err != nil {
				t.Error(err)
			}

			if data == nil {
				t.Error("data should not be nil")
			}
		})
	}
}
