# reportd

[![GoDoc](https://godoc.org/github.com/icco/reportd?status.svg)](https://godoc.org/github.com/icco/reportd) [![Go Report Card](https://goreportcard.com/badge/github.com/icco/reportd)](https://goreportcard.com/report/github.com/icco/reportd)

A service for receiving CSP reports and others.

This service will log the reports recieved from a variety of `report-uri` and `report-to` systems as validated JSON to standard out.

 - CSP: https://www.w3.org/TR/CSP3/
 - Report-To: https://developers.google.com/web/updates/2018/09/reportingapi
 - NEL: https://www.w3.org/TR/network-error-logging/
 - Expect-CT: https://httpwg.org/http-extensions/expect-ct.html
