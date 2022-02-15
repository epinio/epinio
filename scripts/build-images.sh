#!/bin/bash

set -e

version="$(git describe --tags)"
image="splatform/epinio-server"

# Build image
docker build -t "${image}:${version}" -t "${image}:latest" -f images/Dockerfile .
