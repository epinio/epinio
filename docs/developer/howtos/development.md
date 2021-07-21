# Development Guidelines

## Local development environment

### Get a cluster

There are many options on how to get a local cluster for development. Here are a few:

- [k3d](https://k3d.io/)
- [k3s](https://github.com/k3s-io/k3s)
- [kind](https://github.com/kubernetes-sigs/kind)
- [minikube](https://minikube.sigs.k8s.io/docs/start/)

Assuming you have `k3d` installed, you can create a cluster with this command:

```
k3d cluster create epinio
```

This command should automatically update your default kubeconfig to point to
the new cluster but if you need to save your kubeconfig manually you can do it with:

```
k3d kubeconfig get epinio > epinio_kubeconfig
```

### Build Epinio

You can build Epinio with the following make target:

```
make build
```

This is building Epinio for linux on amd64 architecture. If you are on a
different OS or architecture you can use one of the available `build-*` targets.
Look at the Makefile at the root of the project to see what is available.

### Installing Epinio

You can have a look at [the dedicated document](/docs/user/howtos/install.md) for cluster
specific instructions, but generally this should be sufficient to get you running:

```
make install
./dist/epinio-linux-amd64 org create workspace
./dist/epinio-linux-amd64 target workspace
```

In case you're curious why `make install` is used here instead of
`epinio install`, [look behind the curtains](behind-the-curtains.md)
explains the details of running an Epinio dev environment.



After making changes to the binary simply invoking `make
patch-epinio-deployment` again will upload the changes into the
running cluster.

Another thing `epinio install` does after deploying all components is
the creation and targeting of a standard organization, `workspace`.

With the failing server component these actions will fail, making the
installation fail. Using the option `--skip-default-org` instructs the
command to forego these actions. Which in turn makes it necessary to
run them manually to reach the standard state of a cluster. These are
the last two commands in the above script.

The one post-deployment action performed by `install` not affected by
all of the above is the automatic `config update-credentials` saving
API credentials and certs into the client configuration file. As that
command talks directly to the cluster and not the epinio API the
failing server component does not matter.

If the cluster is not running on linux-x64 it may be necessary to set
`EPINIO_BINARY_PATH` to the right binary
([See here](https://github.com/epinio/epinio/blob/2c3c93f79b1019fe7895273b94f40b725ede2996/scripts/patch-epinio-deployment.sh#L19)).

Also, the default `make build` target builds a dynamically linked
binary. This can cause issues if for example the glibc library in the
base image doesn't match the one on the build system. To get past that
issue it is necessary to build a statically linked binary with a
command like:

```
GOARCH="amd64" GOOS="linux" CGO_ENABLED=0 go build -o dist/epinio-linux-amd64
```
