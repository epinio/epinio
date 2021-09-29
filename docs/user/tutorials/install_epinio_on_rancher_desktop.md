# Rancher Desktop configuration

## Rancher Desktop Prerequisites

* Running on Windows requires Windows Subsystem for Linux (WSL) which is automatically installed by Rancher Desktop
* Epinio v0.1.1 has been tested with Rancher Desktop incl. kubernetes version v1.21.5

### Install Rancher Desktop

Install the [latest version](https://github.com/rancher-sandbox/rancher-desktop/releases) from Rancher Desktop for your operating system

## Setup Kubernetes

When running Rancher Desktop for the first time wait until the initialization is completed. Make sure that a supported Kubernetes version is selected under `Kubernetes Settings`, e.g. **v1.21.5**

## Install epinio

Make sure Rancher Desktop is running

### Windows

1. Start a terminal (e.g. type `cmd` in the search field) and change to the directory where `epinio-windows-amd64` is located, e.g. `cd Downloads`

2. Run `epinion-windows-amd64 install`

### Mac

1. Start a terminal and change to the directory where `epinio-darwin-amd64` is located, e.g. `cd Downloads`

2. Run `epinion-darwin-amd64 install`

### Linux

Linux is not yet supported by Rancher Desktop - see https://github.com/rancher-sandbox/rancher-desktop/issues/426
