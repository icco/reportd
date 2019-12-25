FROM golang:1.13-alpine

ENV GOPROXY="https://proxy.golang.org"
ENV GO111MODULE="on"
ENV NAT_ENV="production"
ENV PROJECT="icco-cloud"
ENV DATASET="reportd"
ENV TABLE="reports"

EXPOSE 8080

WORKDIR /go/src/github.com/icco/reportd

RUN apk add --no-cache git
COPY . .

RUN go build -v -o /go/bin/server .

CMD ["/go/bin/server"]
