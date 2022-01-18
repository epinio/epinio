ARG BASE_IMAGE=splatform/epinio-base

################
FROM golang:1.17.5 AS build
WORKDIR /go/src/github.com/epinio

# for caching
COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN LDFLAGS="$LDFLAGS -s -w" make build-linux-amd64

################
FROM $BASE_IMAGE
COPY --from=build /go/src/github.com/epinio/dist/epinio-linux-amd64 /epinio
ENTRYPOINT ["/epinio"]
