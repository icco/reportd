package lib

import (
	"encoding/json"
	"fmt"
	"mime"
	"time"
)

// ExpectCTReport is the struct for Expect-CT errors.
type ExpectCTReport struct {
	ExpectCTReport struct {
		DateTime                  time.Time `json:"date-time"`
		EffectiveExpirationDate   time.Time `json:"effective-expiration-date"`
		Hostname                  string    `json:"hostname"`
		Port                      int       `json:"port"`
		Scts                      []string  `json:"scts"`
		ServedCertificateChain    []string  `json:"served-certificate-chain"`
		ValidatedCertificateChain []string  `json:"validated-certificate-chain"`
	} `json:"expect-ct-report"`
}

// CSPReport is the struct for CSP errors.
type CSPReport struct {
	CSPReport struct {
		DocumentURI        string `json:"document-uri"`
		Referer            string `json:"referrer"`
		ViolatedDirective  string `json:"violated-directive"`
		EffectiveDirective string `json:"effective-directive"`
		OriginalPolicy     string `json:"original-policy"`
		BlockedURI         string `json:"blocked-uri"`
		StatusCode         int    `json:"status-code"`
		SourceFile         string `json:"source-file"`
		LineNumber         int    `json:"line-number"`
		ColumnNumber       int    `json:"column-number"`
	} `json:"csp-report"`
}

// Report is the struct for generic reports via the Reporting API.
type Report struct {
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

// ParseReport takes a content-type header and a body json string and parses it
// into valid Go structs.
func ParseReport(ct, body string) (interface{}, error) {
	media, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return nil, err
	}

	switch media {
	case "application/reports+json":
		var data []Report
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
		return data, nil
		// https://www.w3.org/TR/CSP3/#violation
	case "application/csp-report":
		var data CSPReport
		err := json.Unmarshal([]byte(body), &data)
		if err != nil {
			return nil, err
		}
		return data, nil
	}

	return nil, fmt.Errorf("\"%s\" is not a valid content-type", media)
}
