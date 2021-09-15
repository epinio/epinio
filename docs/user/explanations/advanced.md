# Epinio, Advanced Topics

## Contents

TODO: Do the same for Minio and list storage class requirements (WaitForFirstConsumer). Add links to upstream docs.
TODO: Explain how to configure external S3 storage.
TODO: Consider doing this in a separate document about the components. Or even, one document per components and links here.

- [Epinio installed components](#epinio-installed-components)
  - [Traefik](#traefik)
  - [Linkerd](#linkerd)
  - [Epinio](#epinio)
  - [Cert Manager](#cert-manager)
  - [Kubed](#kubed)
  - [Google Service Broker](#google-service-broker)
  - [Minibroker](#minibroker)
  - [Minio](#minio)
  - [Container Registry](#container-registry)
  - [Service Catalog](#service-catalog)
  - [Tekton](#tekton)

- [Other advanced topics](#other-advanced-topics)
  - [Git Pushing](#git-pushing)
  - [Traefik and Linkerd](#traefik-and-linkerd)

## Epinio installed components

### Traefik

When you installed Epinio, it looked at your cluster to see if you had
[Traefik](https://doc.traefik.io/traefik/providers/kubernetes-ingress/)
running. If it wasn't there it installed it.

As Epinio only checks two namespaces for Traefik's presence, namely
`traefik` and `kube-system`, it is possible that it tries to install
it, despite the cluster having Traefik running. Just in an unexpected
place.

The `install` command provides the option `--skip-traefik` to handle
this kind of situation.

Installing Epinio on your cluster with the command

```bash
$ epinio install --skip-traefik
```

forces Epinio to not install its own Traefik.

Note that having some other (non-Traefik) Ingress controller running
is __not__ a reason to prevent Epinio from installing Traefik. All the
Ingresses used by Epinio expect to be handled by Traefik.

Also, the Traefik instance installed by Epinio, is configured to redirect all
http requests to https automatically (e.g. the requests to the Epinio API, and
the application routes). If you decide to use a Traefik instance which you
deployed, you have to set this up yourself, Epinio won't change your Traefik
installation in any way. Here are the relevant upstream docs for the redirection:

https://doc.traefik.io/traefik/routing/entrypoints/#redirection

### Linkerd

By default, Epinio installs [Linkerd](https://linkerd.io/) on your cluster. The
various namespaces created by Epinio become part of the Linkerd Service Mesh and
thus all communication between pods is secured with mutualTLS.

In some cases you may not want Epinio to install Linkerd, either because you did
that manually before you install Epinio or for other reasons. You can provide
the `--skip-linkerd` flag to the `install` command to prevent Epinio from
installing any of the Linkerd control plane components:

```bash
$ epinio install --skip-linkerd
```

### Epinio

Epinio is a binary that is used:

- as a cli tool, used to push applications, create services etc.
- as the API server component which runs inside the cluster (invoked with the `epinio server` command)
- as the installer that installs the various needed components on the cluster. The
  API server from the previous point is one of them.

Epinio cli functionality is implemented using the endpoints provided by the Epinio API server
component. For example, when the user asks Epinio to "push" and application, the
cli will contact the "Upload", "Stage" and "Deploy" endpoints of the Epinio API to upload the application code,
create a container image for the application using this code and run the application on the cluster.

The Epinio API server is running on the cluster and made accessible using Kubernetes resources like
Deployments, Services,  Ingresses and Secrets.

### Cert Manager

[Upstream documentation](https://cert-manager.io/docs/)

The Cert manager component is deployed by Epinio and use to generate and renew the various Certificates needed in order to
serve the various accessible endpoints over TLS (e.g. the Epinio API server).

Epinio supports various options when it comes to certificate issuers (let's encrypt, private CA, brings your own CA, self signed certs).
Cert Manager simplifies the way we handle the various different certificate issuers within Epinio.

You can read more about certificate issuers here: [certificate issuers documentation](../howtos/certificate_issuers.md)

### Kubed
### Google Service Broker
### Minibroker
### Minio
### Container Registry
### Service Catalog
### Tekton


## Other Advanced Topics

### Git Pushing

The quick way of pushing an application, as explained at
[Quickstart: Push an application](../tutorials/quickstart.md#push-an-application), uses a local
directory containing a checkout of the application's sources.

Internally this is actually [quite complex](detailed-push-process.md). It
involves the creation and upload of a tarball from these sources by the client
to the Epinio server, copying into Epinio's internal (or external) S3 storage,
copying from that storage to a PersistentVolumeClaim to be used in the tekton pipeline for staging,
i.e. compilation and creation of the docker image to be used by the underlying kubernetes cluster.

The process is a bit different when using the Epinio client's "git mode". In
this mode `epinio push` does not take a local directory of sources, but the
location of a git repository holding the sources, and the id of the revision to
use. The client then asks the Epinio server to pull those sources and store them to the
S3 storage. The rest of the process is the same.

The syntax is

```
epinio push NAME GIT-REPOSITORY-URL --git REVISION
```

For comparison all the relevant syntax:

```
epinio push NAME
epinio push NAME DIRECTORY
epinio push NAME GIT-REPOSITORY-URL --git REVISION
```

## Traefik and Linkerd

By default, with Epinio installing both Traefik and Linkerd, Epinio's
installation process ensures that the Traefik pods are included in the Linkerd
mesh, thus ensuring that external communication to applications is secured on
the leg between loadbalancer and application service.

__However__, there are situations where Epinio does not install Traefik.
This can be because the user specified `--skip-traefik`, or because Epinio
detected a running Traefik, thus can forego its own.
The latter is possible, for example, when using `k3d` as cluster foundation.

In these situations the pre-existing Traefik is __not__ part of the Linkerd
mesh. As a consequence the communication from loadbalancer to application
service is not as secure.

While it is possible to fix this, the fix requires access to the cluster in
general, and to Traefik's namespace specifically. In other words, permissions to
annotate the Traefik namespace are needed, as well as permissions to restart the
pods in that namespace. The latter is necessary, because Linkerd is not able to
inject itself into running pods. It can only intercept pod (re)starts.

### Example

Assuming that Traefik's namespace is called `traefik`, with pods
`traefik-6f9cbd9bd4-z4zw8` and `svclb-traefik-q8g75`
the commands

```
kubectl annotate namespace     traefik linkerd.io/inject=enabled
kubectl delete pod --namespace traefik traefik-6f9cbd9bd4-z4zw8 svclb-traefik-q8g75
```

will bring the Traefik in that namespace into Epinio's linkerd mesh.

Note that this recipe also works for a Traefik provided by `k3d`, in the
`kube-system` namespace.

While it is the namespace which is annotated, only restarted pods are affected
by that, i.e. Traefik's pods here. The other system pods continue to run as they
are.
