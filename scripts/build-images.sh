#!/bin/bash

set -e

version="$(git describe --tags)"
base_image="splatform/epinio-base"
server_image="splatform/epinio-server"

# Build base image
docker build -t "${base_image}:${version}" -t "${base_image}:latest" -f images/baseimage-Dockerfile .
docker push "${base_image}:${version}"
docker push "${base_image}:latest"

# Build server image
docker build -t "${server_image}:${version}" -t "${server_image}:latest" --build-arg BASE_IMAGE=${base_image} -f images/Dockerfile .
docker push "${server_image}:${version}"
docker push "${server_image}:latest"
