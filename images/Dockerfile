ARG BASE_IMAGE=splatform/epinio-base

################
FROM ruby:3.1.0 AS tools
WORKDIR /workdir
COPY . .
RUN apt-get update && apt-get install -y python3-venv
RUN ./scripts/tools-install.sh helm && mv output/bin/helm /helm

################
FROM golang:1.17.5 AS build
WORKDIR /go/src/github.com/epinio

# for caching
COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .
COPY --from=tools /helm /usr/bin/helm

RUN make LDFLAGS+="-s -w" build-linux-amd64

################
FROM $BASE_IMAGE
COPY --from=build /go/src/github.com/epinio/dist/epinio-linux-amd64 /epinio
ENTRYPOINT ["/epinio"]
