FROM alpine as certs
RUN apk --update --no-cache add ca-certificates

FROM scratch
COPY --from=certs /etc/ssl/certs /etc/ssl/certs

# default, if running outside of gorelease with a self-compiled binary
ARG DIST_BINARY=dist/epinio-linux-amd64

COPY ${DIST_BINARY} /epinio
ENTRYPOINT ["/epinio"]