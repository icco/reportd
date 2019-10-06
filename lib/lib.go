package lib

import (
	"encoding/json"
	"fmt"
	"mime"
	"time"
)

// Report is a simple interface for types exported by ParseReport.
type Report interface {
	IsReport() bool
}

// ExpectCTReport is the struct for Expect-CT errors.
type ExpectCTReport struct {
	ExpectCTReport ExpectCTSubReport `json:"expect-ct-report"`
}

func (e ExpectCTReport) IsReport() bool {
	return true
}

// ExpectCTSubReport is the internal datastructure of an ExpectCTReport.
type ExpectCTSubReport struct {
	DateTime                  time.Time `json:"date-time"`
	EffectiveExpirationDate   time.Time `json:"effective-expiration-date"`
	Hostname                  string    `json:"hostname"`
	Port                      int       `json:"port"`
	Scts                      []string  `json:"scts"`
	ServedCertificateChain    []string  `json:"served-certificate-chain"`
	ValidatedCertificateChain []string  `json:"validated-certificate-chain"`
}

// ReportToReport is the struct for generic reports via the Reporting API.
type ReportToReport struct {
	Type      string `json:"type"`
	Age       int    `json:"age"`
	URL       string `json:"url"`
	UserAgent string `json:"user_agent"`
	Body      struct {
		Blocked   string `json:"blocked"`
		Directive string `json:"directive"`
		Policy    string `json:"policy"`
		Status    int    `json:"status"`
		Referrer  string `json:"referrer"`
	} `json:"body"`
}

func (r ReportToReport) IsReport() bool {
	return true
}

// ParseReport takes a content-type header and a body json string and parses it
// into valid Go structs.
func ParseReport(ct, body string) ([]Report, error) {
	media, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return nil, err
	}

	switch media {
	case "application/reports+json":
		var data []ReportToReport
		err := json.Unmarshal([]byte(body), &data)
		if err != nil {
			return nil, err
		}
		return data, nil
	case "application/expect-ct-report+json":
		var data ExpectCTReport
		err := json.Unmarshal([]byte(body), &data)
		if err != nil {
			return nil, err
		}
		return []Report{data}, nil
		// https://www.w3.org/TR/CSP3/#violation
		//	case "application/csp-report":
		//		var data CSPReport
		//		err := json.Unmarshal([]byte(body), &data)
		//		if err != nil {
		//			return nil, err
		//		}
		//		return data, nil
	}

	return nil, fmt.Errorf("\"%s\" is not a valid content-type", media)
}
