package lib

import (
	"io/ioutil"
	"path/filepath"
	"strconv"
	"testing"
)

func TestParseAnalyticsParsesWebVitals(t *testing.T) {
	tests := []test{}

	files, err := ioutil.ReadDir("./analytics-examples")
	if err != nil {
		t.Error(err)
	}

	for _, file := range files {
		json, err := ioutil.ReadFile(filepath.Join(".", "analytics-examples", file.Name()))
		if err != nil {
			t.Error(err)
		}

		tests = append(tests, test{
			ContentType: "application/json",
			JSON:        string(json),
		})
	}

	for i, tc := range tests {
		tc := tc
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
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
