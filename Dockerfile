FROM golang:1.25-alpine

ENV GOPROXY="https://proxy.golang.org"
ENV GO111MODULE="on"
ENV NAT_ENV="production"


EXPOSE 8080

WORKDIR /go/src/github.com/icco/reportd

RUN apk add --no-cache git
COPY . .

RUN go build -v -o /go/bin/server .

CMD ["/go/bin/server"]
