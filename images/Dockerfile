ARG BASE_IMAGE=splatform/epinio-base

FROM golang:1.16.3 AS build
WORKDIR /go/src/github.com/carrier

COPY . .
RUN make build-all-small

FROM $BASE_IMAGE
COPY --from=build /go/src/github.com/carrier/dist/carrier-linux-amd64 /carrier
ENTRYPOINT ["/carrier"]
