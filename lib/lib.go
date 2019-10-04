package lib

import (
	"encoding/json"
	"fmt"
	"mime"
	"time"
)

// {"expect-ct-report":{"date-time":"2019-10-04T01:05:38.621Z","effective-expiration-date":"2019-10-04T01:05:38.621Z","hostname":"expect-ct-report.test","port":443,"scts":[],"served-certificate-chain":[],"validated-certificate-chain":[]}}
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

// { "type": "csp", "age": 10, "url": "https://example.com/vulnerable-page/", "user_agent": "Mozilla/5.0 (X11; Linux x86_64; rv:60.0) Gecko/20100101 Firefox/60.0", "body": { "blocked": "https://evil.com/evil.js", "directive": "script-src", "policy": "script-src 'self'; object-src 'none'", "status": 200, "referrer": "https://evil.com/" } }
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
		//	case "application/csp-report":
		//		var data CSPReport
		//		err := json.Unmarshal([]byte(body), &data)
		//		if err != nil {
		//			return nil, err
		//		}
		//		return data, nil
	}

	return nil, fmt.Errorf("not a valid content-type")
}
