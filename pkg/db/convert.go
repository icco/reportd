package db

import (
	"encoding/json"
	"time"

	"github.com/icco/reportd/pkg/analytics"
	"github.com/icco/reportd/pkg/reporting"
	"github.com/icco/reportd/pkg/reportto"
)

func WebVitalFromAnalytics(wv *analytics.WebVital) *WebVital {
	return &WebVital{
		CreatedAt: time.Now(),
		Service:   wv.Service.StringVal,
		Name:      wv.Name,
		Value:     wv.Value,
		Delta:     wv.Delta,
		VitalID:   wv.ID,
		Label:     wv.Label.StringVal,
	}
}

func ReportToEntryFromReport(r *reportto.Report) *ReportToEntry {
	entry := &ReportToEntry{
		CreatedAt: time.Now(),
		Service:   r.Service.StringVal,
	}

	raw, _ := json.Marshal(r)
	entry.RawJSON = string(raw)

	if r.CSP != nil {
		entry.ReportType = "csp"
		entry.DocumentURI = r.CSP.CSPReport.DocumentURI
		entry.BlockedURI = r.CSP.CSPReport.BlockedURI
		entry.ViolatedDirective = r.CSP.CSPReport.ViolatedDirective
		entry.EffectiveDirective = r.CSP.CSPReport.EffectiveDirective
		entry.OriginalPolicy = r.CSP.CSPReport.OriginalPolicy
		entry.SourceFile = r.CSP.CSPReport.SourceFile
		entry.LineNumber = r.CSP.CSPReport.LineNumber
		entry.ColumnNumber = r.CSP.CSPReport.ColumnNumber
		entry.StatusCode = r.CSP.CSPReport.StatusCode
	} else if r.ExpectCT != nil {
		entry.ReportType = "expect-ct"
	} else if len(r.ReportTo) > 0 {
		entry.ReportType = "report-to"
	}

	return entry
}

func SecurityReportEntryFromReport(sr *reporting.SecurityReport) *SecurityReportEntry {
	entry := &SecurityReportEntry{
		CreatedAt: time.Now(),
		Service:   sr.Service.StringVal,
	}

	raw, _ := json.Marshal(sr)
	entry.RawJSON = string(raw)

	if sr.CSP != nil {
		entry.ReportType = "csp-violation"
		entry.URL = sr.CSP.URL
		entry.DocumentURI = sr.CSP.Body.DocumentUri
		entry.BlockedURI = sr.CSP.Body.BlockedUri
		entry.ViolatedDirective = sr.CSP.Body.ViolatedDirective
		entry.EffectiveDirective = sr.CSP.Body.EffectiveDirective
		entry.SourceFile = sr.CSP.Body.SourceFile
		entry.LineNumber = int(sr.CSP.Body.LineNumber)
		entry.ColumnNumber = int(sr.CSP.Body.ColumnNumber)
	} else if sr.Deprecation != nil {
		entry.ReportType = "deprecation"
		entry.URL = sr.Deprecation.URL
	}

	return entry
}
