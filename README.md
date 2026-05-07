# reportd

[![GoDoc](https://godoc.org/github.com/icco/reportd?status.svg)](https://godoc.org/github.com/icco/reportd)
[![Go Report Card](https://goreportcard.com/badge/github.com/icco/reportd)](https://goreportcard.com/report/github.com/icco/reportd)

A self-hosted service for collecting browser security reports and Web Vitals, with a dashboard for understanding your site's health.

## What it does

reportd receives data from two browser APIs and presents it in a dashboard:

- **Web Vitals** (LCP, CLS, INP, FCP, TTFB) -- performance metrics sent by the [web-vitals](https://github.com/GoogleChrome/web-vitals) library
- **Browser Reports** (CSP violations, deprecation warnings, interventions, crashes, COOP/COEP violations, permissions policy violations) -- sent automatically by browsers via the [Reporting API](https://developer.chrome.com/docs/capabilities/web-apis/reporting-api)

Data is stored in a SQL database (**Postgres** or **SQLite**, for fast dashboard queries via GORM) and **BigQuery** (for long-term analytics).

## Supported report types

| Type | Source | Description |
|------|--------|-------------|
| CSP violation | `report-to` / Reporting API | Content Security Policy violations |
| Expect-CT | `report-to` | Certificate Transparency violations (legacy) |
| Deprecation | Reporting API | Usage of deprecated browser APIs |
| Permissions Policy | Reporting API | Blocked access to browser features (camera, geolocation, etc.) |
| Intervention | Reporting API | Browser blocked something for performance/UX |
| Crash | Reporting API | Tab crash (OOM) or unresponsive page |
| COEP | Reporting API | Cross-Origin-Embedder-Policy violations |
| COOP | Reporting API | Cross-Origin-Opener-Policy violations |
| Document Policy | Reporting API | Document-Policy violations |

Unknown report types are stored as raw JSON for forward compatibility.

## Setup

### Configuration

reportd is configured via environment variables (prefix `REPORTD_`) or command-line flags:

| Variable | Flag | Required | Description |
|----------|------|----------|-------------|
| `REPORTD_DATABASE_URL` | `--database_url` | Yes | Database connection string (Postgres or SQLite) |
| `REPORTD_PROJECT` | `--project` | Yes | GCP project ID for BigQuery |
| `REPORTD_DATASET` | `--dataset` | Yes | BigQuery dataset name |
| `REPORTD_ANALYTICS_TABLE` | `--analytics_table` | Yes | BigQuery table for Web Vitals |
| `REPORTD_REPORTS_TABLE` | `--reports_table` | Yes | BigQuery table for Report-To data |
| `REPORTD_REPORTS_V2_TABLE` | `--reports_v2_table` | No | BigQuery table for Reporting API v1 data |
| `PORT` | -- | No | HTTP port (default: 8080) |

### Docker

```bash
docker run -p 8080:8080 \
  -e REPORTD_DATABASE_URL=postgres://user:pass@host/reportd \
  -e REPORTD_PROJECT=my-gcp-project \
  -e REPORTD_DATASET=reporting \
  -e REPORTD_ANALYTICS_TABLE=analytics \
  -e REPORTD_REPORTS_TABLE=reports \
  -e REPORTD_REPORTS_V2_TABLE=reports_v2 \
  ghcr.io/icco/reportd
```

### Local development

```bash
# Start Postgres
docker run -d --name reportd-pg -p 5432:5432 \
  -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=reportd \
  postgres:17

# Run reportd
export REPORTD_DATABASE_URL=postgres://postgres:postgres@localhost/reportd
export REPORTD_PROJECT=my-project
export REPORTD_DATASET=my-dataset
export REPORTD_ANALYTICS_TABLE=analytics
export REPORTD_REPORTS_TABLE=reports
export REPORTD_REPORTS_V2_TABLE=reports_v2
go run main.go
```

For SQLite instead of Postgres:

```bash
export REPORTD_DATABASE_URL=sqlite:///tmp/reportd.db
```

## Integrating with your site

### Web Vitals

Add this snippet to your site to send Web Vitals to reportd:

```html
<script type="module">
  import { onCLS, onINP, onLCP, onFCP, onTTFB } from 'https://unpkg.com/web-vitals@5?module';

  function sendToAnalytics(metric) {
    const body = JSON.stringify(metric);
    (navigator.sendBeacon && navigator.sendBeacon('https://your-reportd-instance/analytics/yoursite', body)) ||
      fetch('https://your-reportd-instance/analytics/yoursite', { body, method: 'POST', keepalive: true });
  }

  onCLS(sendToAnalytics);
  onFCP(sendToAnalytics);
  onINP(sendToAnalytics);
  onLCP(sendToAnalytics);
  onTTFB(sendToAnalytics);
</script>
```

### Browser reports

Add these HTTP headers to your site's responses:

```
Reporting-Endpoints: default="https://your-reportd-instance/reporting/yoursite"
Content-Security-Policy: ...; report-to default;
Cross-Origin-Opener-Policy: same-origin; report-to="default"
Cross-Origin-Embedder-Policy: require-corp; report-to="default"
Permissions-Policy: camera=(), geolocation=(); report-to="default"
```

For legacy `Report-To` support:

```
Report-To: {"group":"default","max_age":10886400,"endpoints":[{"url":"https://your-reportd-instance/report/yoursite"}]}
```

## API reference

### Ingestion (POST)

| Endpoint | Content-Type | Description |
|----------|-------------|-------------|
| `POST /analytics/{service}` | `application/json` | Web Vitals data |
| `POST /report/{service}` | `application/csp-report`, `application/expect-ct-report+json`, `application/reports+json` | Legacy Report-To data |
| `POST /reporting/{service}` | `application/reports+json` | Reporting API v1 data |

### Dashboard (GET)

| Endpoint | Description |
|----------|-------------|
| `GET /` | Service index with health indicators |
| `GET /view/{service}` | Dashboard for a specific service |
| `GET /api/vitals/{service}` | JSON: p75 summaries and daily time series |
| `GET /api/reports/{service}` | JSON: report counts, recent reports, top violated directives |
| `GET /analytics/{service}` | JSON: daily average Web Vitals |
| `GET /reports/{service}` | JSON: daily report counts |
| `GET /services` | JSON: list of all services |
| `GET /healthz` | Health check |

## Dashboard features

The service view page provides:

- **Core Web Vitals cards** with p75 values rated against Google's thresholds (good / needs improvement / poor)
- **Time-series charts** for each metric with threshold bands
- **Report volume chart** showing report counts by type over time
- **Recent CSP violations table** with violated directive, blocked URI, document URI, and source location
- **Recent reports table** for deprecation warnings, interventions, crashes, and other browser reports
- **Top violated directives** bar chart showing the most frequently violated CSP directives
