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
  - [Google Service Broker](#google-service-broker) (optional)
  - [Minibroker](#minibroker) (optional)
  - [Service Catalog](#service-catalog)
  - [Minio](#minio)
  - [Container Registry](#container-registry)
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

The epinio binary is used as:

- a cli tool, used to push applications, create services etc.
- the API server component which runs inside the cluster (invoked with the `epinio server` command)
- the installer that installs the various needed components on the cluster. The
  API server from the previous point is one of them.

Epinio cli functionality is implemented using the endpoints provided by the Epinio API server
component. For example, when the user asks Epinio to "push" an application, the
cli will contact the "Upload", "Stage" and "Deploy" endpoints of the Epinio API to upload the application code,
create a container image for the application using this code and run the application on the cluster.

The Epinio API server is running on the cluster and made accessible using Kubernetes resources like
Deployments, Services,  Ingresses and Secrets.

### Cert Manager

[Upstream documentation](https://cert-manager.io/docs/)

The Cert manager component is deployed by Epinio and used to generate and renew the various Certificates needed in order to
serve the various accessible endpoints over TLS (e.g. the Epinio API server).

Epinio supports various options when it comes to certificate issuers (let's encrypt, private CA, bring your own CA, self signed certs).
Cert Manager simplifies the way we handle the various different certificate issuers within Epinio.

You can read more about certificate issuers here: [certificate issuers documentation](../howtos/certificate_issuers.md)

### Kubed

[Upstream documentation](https://github.com/kubeops/kubed)

Kubed is installed by Epinio to keep secrets that are needed in more than
one namespace synced. For example, the TLS certificate of the [Container Registry](#container-registry) is also needed
in the Tekton staging namespace, in order to trust the certificate when pushing the application images.

Kubed makes sure that if the source secret changes, the copy will change too.

Warning: this doesn't mean things will still work if you re-generate a secret manually. Secret rotation will be handled by Epinio in the future.

### Google Service Broker

[Upstream project link](https://github.com/GoogleCloudPlatform/gcp-service-broker)

Many applications need additional services in order to work. Those services could be databases, cache servers, email gateways, log collecting applications or other.
The various cloud providers already offer such services. Being able to use them in the various Platforms as a Service (PaaS) in a unified way
gave birth to the concept of the (Open Service Broker API - aka osbapi)[https://www.openservicebrokerapi.org/]. This concept is adopted by various
providers among which, Google. There seems to be a shift towards the "Operator" pattern, e.g. Google seems to have moved its focus to the [Config Connector](https://cloud.google.com/config-connector/docs/overview) as described at the top of the README [here](https://github.com/googlearchive/k8s-service-catalog).

Since Google's service broker still works and since the common interface provided by the [Service Catalog](#service-catalog) makes it easy to integrate, Epinio
lets the user optionally install the Google service broker with the command [`epinio enable services-google`](../references/cli/epinio_enable_services-google.md).

Warning: This service broker is not actively supported by Google anymore and may be removed from a future Epinio version.

### Minibroker

[Upstream project link](https://github.com/kubernetes-sigs/minibroker)

This is another implementation of the Open Service Broker API which can also be optionally be installed by epinio with the command [`epinio enable services-incluster`](../references/cli/epinio_enable_services-incluster.md).

Minibroker provisions services on the kubernetes cluster where Epinio is also running, using upstream helm charts. This can be useful for local development with Epinio, because it's fast and doesn't cost anything (services run locally).

### Service Catalog

[Upstream project link](https://github.com/kubernetes-sigs/service-catalog)

The service catalog is the component that collects the various services provided by the various installed service brokers and presents them in a unified way to the user.
The service catalog is the component behind the [`epinio service list-classes`](../references/cli/epinio_service_list-classes.md) and [`epinio service list-plans`](../references/cli/epinio_service_list-plans.md) commands.

### Minio

[Upstream project link](https://github.com/minio/minio)

Minio is a storage solution that implements the same API as [Amazon S3](https://aws.amazon.com/s3/).

When the user pushes an application using a source code directory (with the [`epinio push`](../references/cli/epinio_push.md) command), the first step taken by the cli is to put the source code into a tarball and upload that to the Epinio API server. The server copies that to the configured S3 storage to be used later during the staging of the application.

When installing Epinio, the user can choose to use an external S3 compatible storage or let Epinio install Minio on the cluster ([See here how](TODO)).

### Container Registry

The result of Epinio's application staging is a container image. This image is used to create a Kubernetes deployment to run the application code.
The [Tekton](#tekton) pipeline requires that image to be written to some container registry. Epinio installs a registry on the cluster itself to make the process easy and fast.

Note: In the future, Epinio may offer the option to use an external registry and not install one on the cluster ([GitHub Issue](https://github.com/epinio/epinio/issues/784)).

### Tekton

- [Upstream project link](https://github.com/tektoncd/pipeline)
- [Buildpacks pipeline](https://github.com/tektoncd/catalog/tree/main/task/buildpacks/0.3)

Pushing an application to Kubernetes with Epinio involves various steps. One of them is staging, which is the process that creates a container image out of the application source code using [buildpacks](https://buildpacks.io/). Epinio runs the staging process via a [Tekton Pipeline](https://github.com/tektoncd/catalog/tree/main/task/buildpacks/0.3).

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
