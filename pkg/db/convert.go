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
		CreatedAt:  time.Now(),
		Service:    sr.Service.StringVal,
		ReportType: sr.ReportType,
		RawJSON:    sr.RawJSON,
	}

	switch {
	case sr.CSP != nil:
		entry.URL = sr.CSP.URL
		entry.DocumentURI = sr.CSP.Body.DocumentUri
		entry.BlockedURI = sr.CSP.Body.BlockedUri
		entry.ViolatedDirective = sr.CSP.Body.ViolatedDirective
		entry.EffectiveDirective = sr.CSP.Body.EffectiveDirective
		entry.SourceFile = sr.CSP.Body.SourceFile
		entry.LineNumber = int(sr.CSP.Body.LineNumber)
		entry.ColumnNumber = int(sr.CSP.Body.ColumnNumber)
	case sr.Deprecation != nil:
		entry.URL = sr.Deprecation.URL
		entry.Message = sr.Deprecation.Body.Message
		entry.SourceFile = sr.Deprecation.Body.SourceFile
		entry.LineNumber = int(sr.Deprecation.Body.LineNumber)
		entry.ColumnNumber = int(sr.Deprecation.Body.ColumnNumber)
	case sr.PermissionsPolicy != nil:
		entry.URL = sr.PermissionsPolicy.URL
		entry.Message = sr.PermissionsPolicy.Body.Message
		entry.SourceFile = sr.PermissionsPolicy.Body.SourceFile
		entry.LineNumber = int(sr.PermissionsPolicy.Body.LineNumber)
		entry.ColumnNumber = int(sr.PermissionsPolicy.Body.ColumnNumber)
	case sr.Intervention != nil:
		entry.URL = sr.Intervention.URL
		entry.Message = sr.Intervention.Body.Message
		entry.SourceFile = sr.Intervention.Body.SourceFile
		entry.LineNumber = int(sr.Intervention.Body.LineNumber)
		entry.ColumnNumber = int(sr.Intervention.Body.ColumnNumber)
	case sr.Crash != nil:
		entry.URL = sr.Crash.URL
		entry.Message = sr.Crash.Body.Reason
	case sr.COEP != nil:
		entry.URL = sr.COEP.URL
		entry.BlockedURI = sr.COEP.Body.BlockedURL
		entry.Message = sr.COEP.Body.Disposition
	case sr.COOP != nil:
		entry.URL = sr.COOP.URL
		entry.Message = sr.COOP.Body.EffectivePolicy
	case sr.DocumentPolicy != nil:
		entry.URL = sr.DocumentPolicy.URL
		entry.Message = sr.DocumentPolicy.Body.Message
		entry.SourceFile = sr.DocumentPolicy.Body.SourceFile
		entry.LineNumber = int(sr.DocumentPolicy.Body.LineNumber)
		entry.ColumnNumber = int(sr.DocumentPolicy.Body.ColumnNumber)
	}

	return entry
}
