ARG BASE_IMAGE=splatform/epinio-base

################
FROM golang:1.17.5 AS build
WORKDIR /go/src/github.com/epinio

# for caching
COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN make LDFLAGS+="-s -w" build-linux-amd64

################
FROM $BASE_IMAGE
COPY --from=build /go/src/github.com/epinio/dist/epinio-linux-amd64 /epinio
ENTRYPOINT ["/epinio"]
