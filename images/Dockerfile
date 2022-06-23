FROM alpine as certs
RUN apk --update --no-cache add ca-certificates

FROM scratch
COPY --from=certs /etc/ssl/certs /etc/ssl/certs

# default, if running outside of gorelease with a self-compiled binary
ARG DIST_BINARY=dist/epinio-linux-amd64
ARG SWAGGER_FILE=docs/references/api/swagger.json

COPY ${DIST_BINARY} /epinio
COPY ${SWAGGER_FILE} /swagger.json

ENTRYPOINT ["/epinio"]