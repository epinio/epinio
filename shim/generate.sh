#!/bin/bash

shopt -s expand_aliases
alias swagger="docker run --rm -it -e GOPATH=$HOME/go:/go -v $HOME:$HOME -w $(pwd) quay.io/goswagger/swagger"

swagger generate server -f cc-swagger-v2.yaml -a carrier-shim-cf --exclude-main
sudo chown "$USER" -R restapi
sudo chown "$USER" -R models
