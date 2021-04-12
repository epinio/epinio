ARG BASE_IMAGE=splatform/epinio-base

FROM golang:1.16.3 AS build
WORKDIR /go/src/github.com/epinio

COPY . .
RUN make build-all-small

FROM $BASE_IMAGE
COPY --from=build /go/src/github.com/epinio/dist/epinio-linux-amd64 /epinio
ENTRYPOINT ["/epinio"]
