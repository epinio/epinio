#!/bin/bash

set -e

version="$(git describe --tags)"
imageEpServer="ghcr.io/epinio/epinio-server"
imageUnpacker="ghcr.io/epinio/epinio-unpacker"

# Build images
docker build -t "${imageEpServer}:${version}" -t "${imageEpServer}:latest" -f images/Dockerfile .
docker build -t "${imageUnpacker}:${version}" -t "${imageUnpacker}:latest" -f images/unpacker-Dockerfile .
