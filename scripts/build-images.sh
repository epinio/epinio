#!/bin/bash

set -e

version="$(git describe --tags)"
image="ghcr.io/epinio/epinio-server"

# Build image
docker build -t "${image}:${version}" -f images/Dockerfile .
