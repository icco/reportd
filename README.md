# reportd

[![GoDoc](https://godoc.org/github.com/icco/reportd?status.svg)](https://godoc.org/github.com/icco/reportd)
[![Go Report Card](https://goreportcard.com/badge/github.com/icco/reportd)](https://goreportcard.com/report/github.com/icco/reportd)
[![Build Status](https://app.travis-ci.com/icco/reportd.svg?branch=main)](https://app.travis-ci.com/icco/reportd)

A service for receiving CSP reports and others.

## Report To

This service will log the reports recieved from a variety of `report-uri` and `report-to` systems as validated JSON to standard out.

 - CSP: https://www.w3.org/TR/CSP3/
 - Report-To: https://developers.google.com/web/updates/2018/09/reportingapi
 - NEL: https://www.w3.org/TR/network-error-logging/
 - Expect-CT: https://httpwg.org/http-extensions/expect-ct.html

To start sending reports, target https://reportd.natwelch.com/reports/$yourservicename

## Analytics

This service will log the reports recieved for web-vitals. We have only tested with next.js. See https://nextjs.org/docs/advanced-features/measuring-performance and https://web.dev/vitals/ for more information.

To start sending analytics, target https://reportd.natwelch.com/analytics/$yourservicename
