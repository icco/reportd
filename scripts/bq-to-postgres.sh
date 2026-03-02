#!/usr/bin/env bash
#
# Backport data from BigQuery to Postgres.
#
# Prerequisites:
#   - gcloud CLI authenticated (`gcloud auth login`)
#   - psql available
#   - PGCONN set to a Postgres connection string, e.g.:
#       export PGCONN="postgres://user:pass@host:5432/reportd"
#   - BQ_PROJECT / BQ_DATASET set (defaults: icco-cloud / reportd)
#
# Usage:
#   export PGCONN="postgres://..."
#   bash scripts/bq-to-postgres.sh
#
set -euo pipefail

BQ_PROJECT="${BQ_PROJECT:-icco-cloud}"
BQ_DATASET="${BQ_DATASET:-reportd}"
ANALYTICS_TABLE="${BQ_ANALYTICS_TABLE:-analytics}"
REPORTS_TABLE="${BQ_REPORTS_TABLE:-reports}"
REPORTING_TABLE="${BQ_REPORTING_TABLE:-reporting}"

PGCONN="${PGCONN:-${REPORTD_DATABASE_URL:-}}"
if [[ -z "$PGCONN" ]]; then
  echo "Error: PGCONN (or REPORTD_DATABASE_URL) is not set." >&2
  exit 1
fi

command -v bq >/dev/null 2>&1 || { echo "Error: bq CLI not found. Install the Google Cloud SDK." >&2; exit 1; }
command -v psql >/dev/null 2>&1 || { echo "Error: psql not found." >&2; exit 1; }

# Activate service account if credentials file is available
if [[ -n "${GOOGLE_APPLICATION_CREDENTIALS:-}" && -f "$GOOGLE_APPLICATION_CREDENTIALS" ]]; then
  echo "==> Activating service account..."
  gcloud auth activate-service-account --key-file="$GOOGLE_APPLICATION_CREDENTIALS" --quiet
fi

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "==> Exporting analytics (web vitals) from BigQuery..."
bq query --project_id="$BQ_PROJECT" --use_legacy_sql=false --format=json --max_rows=1000000 \
  "SELECT Name, Value, Delta, ID, Label, CAST(Time AS STRING) AS Time, Service
   FROM \`${BQ_PROJECT}.${BQ_DATASET}.${ANALYTICS_TABLE}\`
   ORDER BY Time ASC" \
  > "$TMPDIR/analytics.json"

ANALYTICS_COUNT=$(jq length "$TMPDIR/analytics.json")
echo "    Found $ANALYTICS_COUNT rows"

if [[ "$ANALYTICS_COUNT" -gt 0 ]]; then
  echo "==> Inserting web_vitals into Postgres..."
  jq -r '.[] | [
    .Name // "",
    (.Value // 0 | tostring),
    (.Delta // 0 | tostring),
    .ID // "",
    .Label // "",
    .Service // "",
    .Time // ""
  ] | @tsv' "$TMPDIR/analytics.json" \
  | while IFS=$'\t' read -r name value delta vital_id label srv ts; do
    # BQ NullDateTime format: YYYY-MM-DDTHH:MM:SS or similar
    # Convert to Postgres-friendly timestamp
    pg_ts="${ts:-$(date -u +%Y-%m-%dT%H:%M:%S)}"
    psql "$PGCONN" -q -c "
      INSERT INTO web_vitals (created_at, service, name, value, delta, vital_id, label)
      VALUES ('${pg_ts}', '${srv}', '${name}', ${value}, ${delta}, '${vital_id}', '${label}')
      ON CONFLICT DO NOTHING;
    "
  done
  echo "    Done."
fi

echo ""
echo "==> Exporting report-to entries from BigQuery..."
bq query --project_id="$BQ_PROJECT" --use_legacy_sql=false --format=json --max_rows=1000000 \
  "SELECT
     CAST(Time AS STRING) AS Time,
     Service,
     CSP,
     ExpectCT,
     ReportTo
   FROM \`${BQ_PROJECT}.${BQ_DATASET}.${REPORTS_TABLE}\`
   ORDER BY Time ASC" \
  > "$TMPDIR/reports.json"

REPORTS_COUNT=$(jq length "$TMPDIR/reports.json")
echo "    Found $REPORTS_COUNT rows"

if [[ "$REPORTS_COUNT" -gt 0 ]]; then
  echo "==> Inserting report_to_entries into Postgres..."
  jq -c '.[]' "$TMPDIR/reports.json" | while IFS= read -r row; do
    srv=$(echo "$row" | jq -r '.Service // ""')
    ts=$(echo "$row" | jq -r '.Time // ""')
    pg_ts="${ts:-$(date -u +%Y-%m-%dT%H:%M:%S)}"

    has_csp=$(echo "$row" | jq '.CSP != null and .CSP != {}')
    has_expectct=$(echo "$row" | jq '.ExpectCT != null and .ExpectCT != {}')

    if [[ "$has_csp" == "true" ]]; then
      raw=$(echo "$row" | jq -c '.' | sed "s/'/''/g")
      doc_uri=$(echo "$row" | jq -r '.CSP.CSPReport.DocumentURI // .CSP.CSPReport."document-uri" // ""' | sed "s/'/''/g")
      blocked_uri=$(echo "$row" | jq -r '.CSP.CSPReport.BlockedURI // .CSP.CSPReport."blocked-uri" // ""' | sed "s/'/''/g")
      violated_dir=$(echo "$row" | jq -r '.CSP.CSPReport.ViolatedDirective // .CSP.CSPReport."violated-directive" // ""' | sed "s/'/''/g")
      effective_dir=$(echo "$row" | jq -r '.CSP.CSPReport.EffectiveDirective // .CSP.CSPReport."effective-directive" // ""' | sed "s/'/''/g")
      original_policy=$(echo "$row" | jq -r '.CSP.CSPReport.OriginalPolicy // .CSP.CSPReport."original-policy" // ""' | sed "s/'/''/g")
      source_file=$(echo "$row" | jq -r '.CSP.CSPReport.SourceFile // .CSP.CSPReport."source-file" // ""' | sed "s/'/''/g")
      line_number=$(echo "$row" | jq -r '.CSP.CSPReport.LineNumber // .CSP.CSPReport."line-number" // 0')
      column_number=$(echo "$row" | jq -r '.CSP.CSPReport.ColumnNumber // .CSP.CSPReport."column-number" // 0')
      status_code=$(echo "$row" | jq -r '.CSP.CSPReport.StatusCode // .CSP.CSPReport."status-code" // 0')
      psql "$PGCONN" -q -c "
        INSERT INTO report_to_entries (created_at, service, report_type, document_uri, blocked_uri,
          violated_directive, effective_directive, original_policy, source_file,
          line_number, column_number, status_code, raw_json)
        VALUES ('${pg_ts}', '${srv}', 'csp', '${doc_uri}', '${blocked_uri}',
          '${violated_dir}', '${effective_dir}', '${original_policy}', '${source_file}',
          ${line_number}, ${column_number}, ${status_code}, '${raw}')
        ON CONFLICT DO NOTHING;
      "
    elif [[ "$has_expectct" == "true" ]]; then
      raw=$(echo "$row" | jq -c '.' | sed "s/'/''/g")
      psql "$PGCONN" -q -c "
        INSERT INTO report_to_entries (created_at, service, report_type, document_uri, blocked_uri,
          violated_directive, effective_directive, original_policy, source_file,
          line_number, column_number, status_code, raw_json)
        VALUES ('${pg_ts}', '${srv}', 'expect-ct', '', '', '', '', '', '', 0, 0, 0, '${raw}')
        ON CONFLICT DO NOTHING;
      "
    else
      # ReportTo is a repeated field — one Postgres row per item
      echo "$row" | jq -c '.ReportTo // [] | .[]' | while IFS= read -r rt; do
        raw=$(echo "$rt" | jq -c '.' | sed "s/'/''/g")
        rt_type=$(echo "$rt" | jq -r '.Type // ""')
        rt_url=$(echo "$rt" | jq -r '.URL // ""' | sed "s/'/''/g")
        doc_uri=$(echo "$rt" | jq -r '.Body.DocumentURL // .Body.documentURL // ""' | sed "s/'/''/g")
        blocked_uri=$(echo "$rt" | jq -r '.Body.BlockedURL // .Body.blockedURL // ""' | sed "s/'/''/g")
        effective_dir=$(echo "$rt" | jq -r '.Body.EffectiveDirective // .Body.effectiveDirective // ""' | sed "s/'/''/g")
        violated_dir=$(echo "$rt" | jq -r '.Body.Directive // .Body.directive // ""' | sed "s/'/''/g")
        original_policy=$(echo "$rt" | jq -r '.Body.OriginalPolicy // .Body.originalPolicy // ""' | sed "s/'/''/g")
        source_file=$(echo "$rt" | jq -r '.Body.SourceFile // .Body.sourceFile // ""' | sed "s/'/''/g")
        line_number=$(echo "$rt" | jq -r '.Body.LineNumber // .Body.lineNumber // 0')
        column_number=$(echo "$rt" | jq -r '.Body.ColumnNumber // .Body.columnNumber // 0')
        status_code=$(echo "$rt" | jq -r '.Body.StatusCode // .Body.status_code // 0')
        # Use URL as document_uri fallback (matches Go logic)
        if [[ -z "$doc_uri" && -n "$rt_url" ]]; then
          doc_uri="$rt_url"
        fi
        psql "$PGCONN" -q -c "
          INSERT INTO report_to_entries (created_at, service, report_type, document_uri, blocked_uri,
            violated_directive, effective_directive, original_policy, source_file,
            line_number, column_number, status_code, raw_json)
          VALUES ('${pg_ts}', '${srv}', '${rt_type}', '${doc_uri}', '${blocked_uri}',
            '${violated_dir}', '${effective_dir}', '${original_policy}', '${source_file}',
            ${line_number}, ${column_number}, ${status_code}, '${raw}')
          ON CONFLICT DO NOTHING;
        "
      done
    fi
  done
  echo "    Done."
fi

echo ""
echo "==> Exporting reporting (security reports v2) from BigQuery..."
bq query --project_id="$BQ_PROJECT" --use_legacy_sql=false --format=json --max_rows=1000000 \
  "SELECT
     CAST(Time AS STRING) AS Time,
     Service,
     ReportType,
     Deprecation,
     CSP,
     PermissionsPolicy,
     Intervention,
     Crash,
     COEP,
     COOP,
     DocumentPolicy
   FROM \`${BQ_PROJECT}.${BQ_DATASET}.${REPORTING_TABLE}\`
   ORDER BY Time ASC" \
  > "$TMPDIR/reporting.json"

REPORTING_COUNT=$(jq length "$TMPDIR/reporting.json")
echo "    Found $REPORTING_COUNT rows"

if [[ "$REPORTING_COUNT" -gt 0 ]]; then
  echo "==> Inserting security_report_entries into Postgres..."
  jq -c '.[]' "$TMPDIR/reporting.json" | while IFS= read -r row; do
    srv=$(echo "$row" | jq -r '.Service // ""')
    ts=$(echo "$row" | jq -r '.Time // ""')
    pg_ts="${ts:-$(date -u +%Y-%m-%dT%H:%M:%S)}"
    report_type=$(echo "$row" | jq -r '.ReportType // ""')
    raw=$(echo "$row" | jq -c '.' | sed "s/'/''/g")

    url=""
    doc_uri=""
    blocked_uri=""
    violated_dir=""
    effective_dir=""
    source_file=""
    line_number=0
    column_number=0
    message=""

    case "$report_type" in
      csp-violation)
        url=$(echo "$row" | jq -r '.CSP.URL // ""' | sed "s/'/''/g")
        doc_uri=$(echo "$row" | jq -r '.CSP.Body.DocumentUri // .CSP.Body.document_uri // ""' | sed "s/'/''/g")
        blocked_uri=$(echo "$row" | jq -r '.CSP.Body.BlockedUri // .CSP.Body.blocked_uri // ""' | sed "s/'/''/g")
        violated_dir=$(echo "$row" | jq -r '.CSP.Body.ViolatedDirective // .CSP.Body.violated_directive // ""' | sed "s/'/''/g")
        effective_dir=$(echo "$row" | jq -r '.CSP.Body.EffectiveDirective // .CSP.Body.effective_directive // ""' | sed "s/'/''/g")
        source_file=$(echo "$row" | jq -r '.CSP.Body.SourceFile // .CSP.Body.source_file // ""' | sed "s/'/''/g")
        line_number=$(echo "$row" | jq -r '.CSP.Body.LineNumber // .CSP.Body.line_number // 0')
        column_number=$(echo "$row" | jq -r '.CSP.Body.ColumnNumber // .CSP.Body.column_number // 0')
        ;;
      deprecation)
        url=$(echo "$row" | jq -r '.Deprecation.URL // ""' | sed "s/'/''/g")
        message=$(echo "$row" | jq -r '.Deprecation.Body.Message // .Deprecation.Body.message // ""' | sed "s/'/''/g")
        source_file=$(echo "$row" | jq -r '.Deprecation.Body.SourceFile // .Deprecation.Body.source_file // ""' | sed "s/'/''/g")
        line_number=$(echo "$row" | jq -r '.Deprecation.Body.LineNumber // .Deprecation.Body.line_number // 0')
        column_number=$(echo "$row" | jq -r '.Deprecation.Body.ColumnNumber // .Deprecation.Body.column_number // 0')
        ;;
      permissions-policy-violation)
        url=$(echo "$row" | jq -r '.PermissionsPolicy.URL // ""' | sed "s/'/''/g")
        message=$(echo "$row" | jq -r '.PermissionsPolicy.Body.Message // .PermissionsPolicy.Body.message // ""' | sed "s/'/''/g")
        source_file=$(echo "$row" | jq -r '.PermissionsPolicy.Body.SourceFile // .PermissionsPolicy.Body.sourceFile // ""' | sed "s/'/''/g")
        line_number=$(echo "$row" | jq -r '.PermissionsPolicy.Body.LineNumber // .PermissionsPolicy.Body.lineNumber // 0')
        column_number=$(echo "$row" | jq -r '.PermissionsPolicy.Body.ColumnNumber // .PermissionsPolicy.Body.columnNumber // 0')
        ;;
      intervention)
        url=$(echo "$row" | jq -r '.Intervention.URL // ""' | sed "s/'/''/g")
        message=$(echo "$row" | jq -r '.Intervention.Body.Message // .Intervention.Body.message // ""' | sed "s/'/''/g")
        source_file=$(echo "$row" | jq -r '.Intervention.Body.SourceFile // .Intervention.Body.sourceFile // ""' | sed "s/'/''/g")
        line_number=$(echo "$row" | jq -r '.Intervention.Body.LineNumber // .Intervention.Body.lineNumber // 0')
        column_number=$(echo "$row" | jq -r '.Intervention.Body.ColumnNumber // .Intervention.Body.columnNumber // 0')
        ;;
      crash)
        url=$(echo "$row" | jq -r '.Crash.URL // ""' | sed "s/'/''/g")
        message=$(echo "$row" | jq -r '.Crash.Body.Reason // .Crash.Body.reason // ""' | sed "s/'/''/g")
        ;;
      coep)
        url=$(echo "$row" | jq -r '.COEP.URL // ""' | sed "s/'/''/g")
        blocked_uri=$(echo "$row" | jq -r '.COEP.Body.BlockedURL // .COEP.Body.blockedURL // ""' | sed "s/'/''/g")
        message=$(echo "$row" | jq -r '.COEP.Body.Disposition // .COEP.Body.disposition // ""' | sed "s/'/''/g")
        ;;
      coop)
        url=$(echo "$row" | jq -r '.COOP.URL // ""' | sed "s/'/''/g")
        message=$(echo "$row" | jq -r '.COOP.Body.EffectivePolicy // .COOP.Body.effectivePolicy // ""' | sed "s/'/''/g")
        ;;
      document-policy-violation)
        url=$(echo "$row" | jq -r '.DocumentPolicy.URL // ""' | sed "s/'/''/g")
        message=$(echo "$row" | jq -r '.DocumentPolicy.Body.Message // .DocumentPolicy.Body.message // ""' | sed "s/'/''/g")
        source_file=$(echo "$row" | jq -r '.DocumentPolicy.Body.SourceFile // .DocumentPolicy.Body.sourceFile // ""' | sed "s/'/''/g")
        line_number=$(echo "$row" | jq -r '.DocumentPolicy.Body.LineNumber // .DocumentPolicy.Body.lineNumber // 0')
        column_number=$(echo "$row" | jq -r '.DocumentPolicy.Body.ColumnNumber // .DocumentPolicy.Body.columnNumber // 0')
        ;;
    esac

    psql "$PGCONN" -q -c "
      INSERT INTO security_report_entries (created_at, service, report_type, url, document_uri,
        blocked_uri, violated_directive, effective_directive, source_file,
        line_number, column_number, message, raw_json)
      VALUES ('${pg_ts}', '${srv}', '${report_type}', '${url}', '${doc_uri}',
        '${blocked_uri}', '${violated_dir}', '${effective_dir}', '${source_file}',
        ${line_number}, ${column_number}, '${message}', '${raw}')
      ON CONFLICT DO NOTHING;
    "
  done
  echo "    Done."
fi

echo ""
echo "==> Migration complete."
echo "    web_vitals:              $ANALYTICS_COUNT rows exported"
echo "    report_to_entries:       $REPORTS_COUNT rows exported"
echo "    security_report_entries: $REPORTING_COUNT rows exported"
