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

func ReportToEntriesFromReport(r *reportto.Report) []*ReportToEntry {
	now := time.Now()
	srv := r.Service.StringVal

	if r.CSP != nil {
		raw, _ := json.Marshal(r)
		return []*ReportToEntry{{
			CreatedAt:          now,
			Service:            srv,
			ReportType:         "csp",
			DocumentURI:        r.CSP.CSPReport.DocumentURI,
			BlockedURI:         r.CSP.CSPReport.BlockedURI,
			ViolatedDirective:  r.CSP.CSPReport.ViolatedDirective,
			EffectiveDirective: r.CSP.CSPReport.EffectiveDirective,
			OriginalPolicy:     r.CSP.CSPReport.OriginalPolicy,
			SourceFile:         r.CSP.CSPReport.SourceFile,
			LineNumber:         r.CSP.CSPReport.LineNumber,
			ColumnNumber:       r.CSP.CSPReport.ColumnNumber,
			StatusCode:         r.CSP.CSPReport.StatusCode,
			RawJSON:            string(raw),
		}}
	}

	if r.ExpectCT != nil {
		raw, _ := json.Marshal(r)
		return []*ReportToEntry{{
			CreatedAt:  now,
			Service:    srv,
			ReportType: "expect-ct",
			RawJSON:    string(raw),
		}}
	}

	var entries []*ReportToEntry
	for _, rt := range r.ReportTo {
		raw, _ := json.Marshal(rt)
		entry := &ReportToEntry{
			CreatedAt:          now,
			Service:            srv,
			ReportType:         rt.Type,
			DocumentURI:        rt.Body.DocumentURL,
			BlockedURI:         rt.Body.BlockedURL,
			EffectiveDirective: rt.Body.EffectiveDirective,
			OriginalPolicy:     rt.Body.OriginalPolicy,
			SourceFile:         rt.Body.SourceFile,
			LineNumber:         int(rt.Body.LineNumber),
			ColumnNumber:       int(rt.Body.ColumnNumber),
			StatusCode:         int(rt.Body.StatusCode),
			RawJSON:            string(raw),
		}
		if rt.Body.Directive != "" {
			entry.ViolatedDirective = rt.Body.Directive
		}
		if rt.URL != "" {
			entry.DocumentURI = rt.URL
		}
		entries = append(entries, entry)
	}
	return entries
}

func SecurityReportEntryFromReport(sr *reporting.SecurityReport) *SecurityReportEntry {
	entry := &SecurityReportEntry{
		CreatedAt:  time.Now(),
		Service:    sr.Service.StringVal,
		ReportType: sr.ReportType.StringVal,
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
