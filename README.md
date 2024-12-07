# reportd

[![GoDoc](https://godoc.org/github.com/icco/reportd?status.svg)](https://godoc.org/github.com/icco/reportd) 
[![Go Report Card](https://goreportcard.com/badge/github.com/icco/reportd)](https://goreportcard.com/report/github.com/icco/reportd)

A service for receiving CSP reports and others.

## Report To

This service will log the reports recieved from a variety of `report-uri` and `report-to` systems as validated JSON to standard out.

 - CSP: https://www.w3.org/TR/CSP3/
 - Report-To: https://developers.google.com/web/updates/2018/09/reportingapi
 - NEL: https://www.w3.org/TR/network-error-logging/
 - Expect-CT: https://httpwg.org/http-extensions/expect-ct.html

To start sending reports, target https://reportd.natwelch.com/reports/$yourservicename

### TODO

We need to add support for the following:

 - COEP: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Cross-Origin-Embedder-Policy
 - COOP: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Cross-Origin-Opener-Policy

Also need to add support for Reporting API v1. see migration policy https://developer.chrome.com/blog/reporting-api-migration

## Analytics

This service will log the reports recieved for web-vitals. We have only tested with next.js. See https://nextjs.org/docs/advanced-features/measuring-performance and https://web.dev/vitals/ for more information.

To start sending analytics, target https://reportd.natwelch.com/analytics/$yourservicename

```html
    <script type="module">
      import { onCLS, onINP, onLCP, onFCP, onFID, onTTFB } from 'https://unpkg.com/web-vitals@4?module';

      function sendToAnalytics(metric) {
        const body = JSON.stringify(metric);
        (navigator.sendBeacon && navigator.sendBeacon('https://reportd.natwelch.com/analytics/$yourservicename', body)) ||
          fetch('https://reportd.natwelch.com/analytics/$yourservicename', { body, method: 'POST', keepalive: true });
      }

      onCLS(sendToAnalytics);
      onFCP(sendToAnalytics);
      onFID(sendToAnalytics);
      onINP(sendToAnalytics);
      onLCP(sendToAnalytics);
      onTTFB(sendToAnalytics);
    </script>
```
