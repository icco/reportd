package lib

import (
	"reflect"
	"strconv"
	"testing"
	"time"
)

type test struct {
	ContentType string
	JSON        string
	Expect      *Report
}

func TestParseReport(t *testing.T) {
	tests := []test{
		test{
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

			if reflect.DeepEqual(data, tc.Expect) {
				t.Errorf("data is not accurate: %+v != %+v", data, tc.Expect)
			}
		})
	}
}
