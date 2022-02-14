#!/bin/bash

set -e

version="$(git describe --tags)"
base_image="splatform/epinio-base"
server_image="splatform/epinio-server"

# Build base image
docker build -t "${base_image}:${version}" -t "${base_image}:latest" -f images/baseimage-Dockerfile .

# Build server image
docker build -t "${server_image}:${version}" -t "${server_image}:latest" --build-arg BASE_IMAGE=${base_image} -f images/Dockerfile .
