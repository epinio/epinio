# Development Guidelines

## Local development environment

An Epinio installation consists of various components which are usually deployed
using the [official Epinio helm chart](https://github.com/epinio/helm-charts/tree/main/chart/epinio).
For convenience and in order to be able to point to a specific commit of the helm-chart
repository, one that works with the current commit in the `epinio/epinio` repository,
the helm-chart repository is a git submodule in the `epinio/epinio` one.

In order to work on the Epinio code, Epinio needs to be installed using the
helm-chart in the submodule, the binary needs to be compiled from source and
then the epinio-server deployment that runs in the cluster needs to be updated to
run the newly compiled binary. The binary is both the epinio API server and the
Epinio cli used to interact with the API server.

There are various `make` targets that help prepare a development environment.
They are described in order below.

NOTE: Most scripts assume they run on a Linux OS. They may have to be adapted in
      order to work on another OS.

These are the basic steps for installing from source.

1. Clone the epinio/epinio repository.
2. Get the chart submodule. `git submodule init` and `git submodule update`.
3. If you already have a cluster, you can skip the k3d steps.
4. Install pre-requisites (cert-manager)
5. Install Epinio

## Detailed instructions

### 1. Creating a cluster

If you're developing against an existing cluster (for example, using [Rancher Desktop](https://rancherdesktop.io/)), you can skip this step.

There are many options on how to get a local cluster for development. Here are a few:

- [k3d](https://k3d.io/)
- [k3s](https://github.com/k3s-io/k3s)
- [kind](https://github.com/kubernetes-sigs/kind)
- [minikube](https://minikube.sigs.k8s.io/docs/start/)

Assuming you have `k3d` installed, you can create a cluster with this `make` target :

```bash
make acceptance-cluster-setup
```

(as the name also implies, this target is also used to prepare a cluster for the acceptance test suite in CI)

This command writes the kubeconfig file to talk to the cluster in `tmp/acceptance-kubeconfig`.
For the following steps to work, `KUBECONFIG` needs to be exported as so:


```bash
export KUBECONFIG=$PWD/tmp/acceptance-kubeconfig
```

Alternatively, the following command will merge the current context in the current
configuration and make it the default context (don't set the `KUBECONFIG`
variable with the above command if you want to update the default configuration).
This way, the `KUBECONFIG` variable won't have to be exported in every virtual terminal.

```bash
k3d kubeconfig merge -d epinio-acceptance
```

### 2. Install cert-manager

[Cert Manager](https://cert-manager.io/) is an external dependency of Epinio and
is not installed by the official helm-chart. There is a `make` target that will
install cert-manager on the cluster to be used by the Epinio installation later:

```bash
make install-cert-manager
```

### Install Epinio

The following make target will use the helm-chart from the git submodule directory,
to install Epinio on the cluster:

If developing with k3d, you can use the following command without extras:

```bash
make prepare_environment_k3d
```

If developing against a known environment (such as k3s with Rancher Desktop), you may want to specify the ingress. In that case:

```bash
EPINIO_SYSTEM_DOMAIN=localhost make prepare_environment_k3d
```

### Run the current development build

Every time a change is made in the Epinio source code, the binary running inside
the epinio-server Pod has to be replaced with a freshly compiled one. This can
be achieved by running the following command:

```bash
make && make patch-epinio-deployment
```

This first compiles a new binary locally and then replaces the one running inside the Pod with it.

If the cluster is not running on linux-amd64 it will be necessary to set
`EPINIO_BINARY_PATH` to the correct binary to place into the epinio server
([See here](https://github.com/epinio/epinio/blob/a4b679af88d58177cecf4a5717c8c96f382058ed/scripts/patch-epinio-deployment.sh#L19)).

If the client operation is performed outside of a git checkout it will be
necessary to set `EPINIO_BINARY_TAG` to the correct tag
([See here](https://github.com/epinio/epinio/blob/a4b679af88d58177cecf4a5717c8c96f382058ed/scripts/patch-epinio-deployment.sh#L20)).

The make target `tag` can be used in the checkout the binary came from to
determine this value.

Also, the default `make build` target builds a dynamically linked
binary. This can cause issues if for example the glibc library in the
base image doesn't match the one on the build system. To get past that
issue it is necessary to build a statically linked binary with a
command like:

```
GOARCH="amd64" GOOS="linux" CGO_ENABLED=0 go build -o dist/epinio-linux-amd64
```

#### Mixed Windows/Linux Scenario

A concrete example of the above would be the installation of Epinio from a
Windows host without a checkout, to a Linux-based cluster.

In that scenario the Windows host has to have both windows-amd64 and linux-amd64
binaries. The first to perform the installation, the second for
`EPINIO_BINARY_PATH` to be put into the server.

Furthermore, as the Windows host is without a checkout, the tag has to be
determined in the actual checkout and set into `EPINIO_BINARY_PATH`.

Lastly, do not forget to set up a proper domain so that the client can talk to
the server after installation is done. While during installation only a suitable
`KUBECONFIG` is required after the client will go and use the information from
the ingress, and that then has to properly resolve in the DNS.

## Cleanup

There are several cleanup steps available in the makefile.

To "uninstall" the Epinio dev instance:

```bash
make unprepare_environment_k3d
```

To delete the k3d cluster:

```bash
make acceptance-cluster-delete
```
