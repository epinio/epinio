# Rancher Desktop configuration

## Rancher Desktop Prerequisites

* Running on Windows requires Windows Subsystem for Linux (WSL) which is automatically installed by Rancher Desktop

### Install Rancher Desktop

Install the [latest version](https://github.com/rancher-sandbox/rancher-desktop/releases) from Rancher Desktop for your operating system

## Setup Kubernetes

When running Rancher Desktop for the first time wait until the initialization is completed. Make sure that a supported Kubernetes version is selected under `Kubernetes Settings`, e.g. **v1.21.5**

## Install epinio

Make sure Rancher Desktop is running.

Rancher Desktop can report Kubernetes as running while some pods are actually not yet ready.
Manual verification is possible by executing the command `kubectl get pods -A` in a terminal and checking that all pods report either `Running` or `Completed` as their status.

### Windows

1. Start a terminal (e.g. type `cmd` in the search field) and change to the directory where `epinio-windows-amd64` is located, e.g. `cd Downloads`

2. Run `epinion-windows-amd64 install`

3. Copy the binary to a directory of your choice and add it to the `PATH` variable as described by [Kevin Berg's article](https://medium.com/@kevinmarkvi/how-to-add-executables-to-your-path-in-windows-5ffa4ce61a53). This allows execution of Epinio directly from any terminal.

### Mac

1. Start a terminal and change to the directory where `epinio-darwin-amd64` is located, e.g. `cd Downloads`

2. Run `epinion-darwin-amd64 install`

### Linux

Linux is not yet supported by Rancher Desktop - see https://github.com/rancher-sandbox/rancher-desktop/issues/426
