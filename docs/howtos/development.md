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
different OS or architecture you can use one of other the available `build-*` targets.
Look at the Makefile at the root of the project to see what is available.

### Installing Epinio

You can have a look at [the dedicated document](/docs/user/howtos/install.md) for cluster
specific instructions, but generally this should be sufficient to get you running:

```
make install
./dist/epinio-linux-amd64 namespace create workspace
./dist/epinio-linux-amd64 target workspace
```

In case you're curious why `make install` is used here instead of
`epinio install`, we have a section to look
[Behind the curtains](#curtain) at the end of the document, which
explains the details of running an Epinio dev environment.

After making changes to the binary simply invoking `make
patch-epinio-deployment` again will upload the changes into the
running cluster.

Another thing `epinio install` does after deploying all components is
the creation and targeting of a standard namespace, `workspace`.

With the failing server component these actions will fail, making the
installation fail. Using the option `--skip-default-namespace` instructs the
command to forego these actions. Which in turn makes it necessary to
run them manually to reach the standard state of a cluster. These are
the last two commands in the above script.

The one post-deployment action performed by `install` not affected by
all of the above is the automatic `config update-credentials` saving
API credentials and certs into the client configuration file. As that
command talks directly to the cluster and not the epinio API the
failing server component does not matter.

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

<a id='curtain'>
#### Behind the curtains

`make install` does quite a bit more than the plain

```
epinio install
```

found in the quick install intructions.

Let's look at what `make install` actually does:

When building Epinio, the generated binary assumes that there is a
container image for the Epinio server components, with a tag that
matches the commit you built from.  For example, when calling `make
build` on commit `7bfb700`, the version reported by Epinio is
something like `v0.0.5-75-g7bfb700` and an image `epinio/server:v0.0.5-75-g7bfb700`
is expected to be found.

This works fine for released versions, because the release pipeline ensures
that such an image is built and published.

However when building locally building and publishing an image for
every little change is ... inconvenient.

`make install` is setting
```
export EPINIO_DONT_WAIT_FOR_DEPLOYMENT=1
```

before calling the `epinio` binary that was created during `make build`.

This tells the `epinio install` command to not wait for the Epinio server
deployment. Since that will be failing without the image. Inspecting
the cluster with

```
kubectl get pod -n epinio --selector=app.kubernetes.io/name=epinio-server
```

will confirm this.

Then `make install` runs `scripts/patch-epinio-deployment.sh` which compensates for this
issue. This make target patches the failing Epinio server deployment
to use an existing image from some release and then copies the locally
built `dist/epinio-linux-amd64` binary into it, ensuring that it runs
the same binary as the client.

__Note__ When building for another OS or architecture the
`dist/epinio-linux-amd64` binary will not exist, and the script has to
be adjusted accordingly.
