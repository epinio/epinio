# Development Guidelines

## Commit Message Guidelines:

All new commits need to follow the Conventional Commit guidelines:
https://www.conventionalcommits.org/en/v1.0.0/#summary

### Setup a git hook for commit linting

We use [gitlint](https://jorisroovers.com/gitlint/) to lint all new commits.

The hook can be setup by running the following command inside of the `epinio` git directory:

```bash
gitlint install-hook
```

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

You can have a look at [the dedicated document](/docs/install.md) for cluster
specific instructions, but generally this should be sufficient to get you running:

```
./dist/epinio-linux-amd64 install
```

When you build Epinio, the binary will assumes there is a container image for
the Epinio components with a tag that matches the commit you built from.
For example, when calling `make build` on commit `7bfb700`, the version reported
by Epinio is `v0.0.5-75-g7bfb700` and an image `epinio/server:v0.0.5-75-g7bfb700`
is expected.

This works fine for released version, because the pipeline ensures such an image
is built and published. But when you are building locally you don't want to
build and publish an image for every little change you make. For that reason
you can tell `epinio install` command to not wait for the epinio server deployment
(since it will be failing) by setting the EPINIO_DONT_WAIT_FOR_DEPLOYMENT environment
variable:

```
export EPINIO_DONT_WAIT_FOR_DEPLOYMENT=1
```

When you run `epinio install` now, it will deploy the epinio server, but if you
inspect the cluster you can see it is failing to start because the image does not
exist:

```
kubectl get pod -n epinio --selector=app.kubernetes.io/name=epinio-server
```

To fix this, just call `make patch-epinio-deployment`. This make target will
patch the epinio server deployment to use an existing image and will copy
the file `dist/epinio-linux-amd64` inside the image making sure you run the same
binary you built locally.

If you built for another OS or architecture then `dist/epinio-linux-amd64` may
not exist so adjust the script accordingly.

If you make changes to your binary you can upload your new built by simply calling
`make patch-epinio-deployment` again.

If your cluster is not running on linux-x64 you may need to set `EPINIO_BINARY_PATH` to the right binary ([See here](https://github.com/epinio/epinio/blob/2c3c93f79b1019fe7895273b94f40b725ede2996/scripts/patch-epinio-deployment.sh#L19)). Also, the default `make build` target builds a dynamically linked binary. This can cause issued if for example the glibc library in the base image doesn't match the one on your system. To get past that issue, you can build a statically linked binary with something like this:

```
GOARCH="amd64" GOOS="linux" CGO_ENABLED=0 go build -o dist/epinio-linux-amd64
```
