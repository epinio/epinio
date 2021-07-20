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
