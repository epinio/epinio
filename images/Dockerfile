ARG BASE_IMAGE=splatform/epinio-base

################
FROM $BASE_IMAGE
# default, if running outside of gorelease with a self-compiled binary
ARG DIST_BINARY=dist/epinio-linux-amd64

COPY ${DIST_BINARY} /epinio
ENTRYPOINT ["/epinio"]
