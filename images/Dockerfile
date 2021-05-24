ARG BASE_IMAGE=splatform/epinio-base

FROM ruby:3.0.1 AS tools
COPY . .
RUN apt-get update && apt-get install -y python3-venv
RUN ./scripts/tools-install.sh helm
RUN cp output/bin/helm /helm

FROM golang:1.16.3 AS build
WORKDIR /go/src/github.com/epinio

COPY . .
COPY --from=tools /helm /usr/bin/helm

RUN make build-all-small

FROM $BASE_IMAGE
COPY --from=build /go/src/github.com/epinio/dist/epinio-linux-amd64 /epinio
ENTRYPOINT ["/epinio"]
