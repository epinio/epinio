FROM golang:1.18

RUN go install github.com/onsi/ginkgo/v2/ginkgo@latest

WORKDIR /go/src/epinio
