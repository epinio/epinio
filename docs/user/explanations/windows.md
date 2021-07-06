# Epinio On Windows

Epinio relies on a number of command line tools which are normally
available on any kind of Unix platform, yet rarely on Windows.

The general set contains

  - sh
  - sed
  - git

while the specific set consists of

  - kubectl
  - helm

We are currently recommending to install the
[Git For Windows](https://gitforwindows.org/) distribution as it
provides everything needed from the general set, and more.

For `helm`, `kubectl`, and `epinio` itself the necessary binaries can
be retrieved from the relevant release pages or per their
instructions:

  - [Epinio Releases](https://github.com/epinio/epinio/releases)
  - [Helm Releases](https://github.com/helm/helm/releases)
  - [Kubectl Instructions](https://kubernetes.io/docs/tasks/tools/install-kubectl-windows/)
