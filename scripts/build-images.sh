#!/bin/bash

set -e

version="$(git describe --tags)"
base_image="splatform/epinio-base:${version}"
server_image="splatform/epinio-server:${version}"

docker build -t ${base_image} -f images/baseimage-Dockerfile .
docker push ${base_image}
docker build -t ${server_image} --build-arg BASE_IMAGE=${base_image} -f images/Dockerfile .
docker push ${server_image}
