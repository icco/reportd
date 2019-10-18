package lib

import (
	"encoding/json"
	"fmt"
	"mime"
	"time"
)

// Report is a simple interface for types exported by ParseReport.
type Report struct {
	ExpectCT *ExpectCTReport
	CSP      *CSPReport
	ReportTo *[]ReportToReport
}

// ExpectCTReport is the struct for Expect-CT errors.
type ExpectCTReport struct {
	ExpectCTReport ExpectCTSubReport `json:"expect-ct-report"`
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

// CSPReport is the struct for CSP errors.
// Spec is at https://www.w3.org/TR/CSP3/#violation.
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

// ReportToReport is the struct for generic reports via the Reporting API.
// TODO: There are multiple ways browsers send the field statuscode!
type ReportToReport struct {
	Type      string `json:"type"`
	Age       int    `json:"age"`
	URL       string `json:"url"`
	UserAgent string `json:"user_agent"`
	Body      struct {
		AnticipatedRemoval int64   `json:"anticipatedRemoval,omitempty"`
		Blocked            string  `json:"blocked,omitempty"`
		BlockedURL         string  `json:"blockedURL,omitempty"`
		ColumnNumber       string  `json:"columnNumber,omitempty"`
		Directive          string  `json:"directive,omitempty"`
		Disposition        string  `json:"disposition,omitempty"`
		DocumentURL        string  `json:"documentURL,omitempty"`
		EffectiveDirective string  `json:"effectiveDirective,omitempty"`
		ElapsedTime        int64   `json:"elapsed_time,omitempty"`
		ID                 string  `json:"id,omitempty"`
		LineNumber         int64   `json:"lineNumber,omitempty"`
		Message            string  `json:"message,omitempty"`
		Method             string  `json:"method,omitempty"`
		OriginalPolicy     string  `json:"originalPolicy,omitempty"`
		Phase              string  `json:"phase,omitempty"`
		Policy             string  `json:"policy,omitempty"`
		Protocol           string  `json:"protocol,omitempty"`
		Reason             string  `json:"reason,omitempty"`
		Referrer           string  `json:"referrer,omitempty"`
		SamplingFraction   float64 `json:"sampling_fraction,omitempty"`
		ServerIP           string  `json:"server_ip,omitempty"`
		SourceFile         string  `json:"sourceFile,omitempty"`
		Status             int64   `json:"status,omitempty"`
		StatusCode         int64   `json:"status_code,omitempty"`
		Type               string  `json:"type,omitempty"`
	} `json:"body"`
}

// ParseReport takes a content-type header and a body json string and parses it
// into valid Go structs.
func ParseReport(ct, body string) (*Report, error) {
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
		return &Report{ReportTo: &data}, nil
	case "application/expect-ct-report+json":
		var data ExpectCTReport
		err := json.Unmarshal([]byte(body), &data)
		if err != nil {
			return nil, err
		}
		return &Report{ExpectCT: &data}, nil
	case "application/csp-report":
		var data CSPReport
		err := json.Unmarshal([]byte(body), &data)
		if err != nil {
			return nil, err
		}
		return &Report{CSP: &data}, nil
	}

	return nil, fmt.Errorf("\"%s\" is not a valid content-type", media)
}
