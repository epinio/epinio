# Epinio, Advanced Topics

Opinionated platform that runs on Kubernetes, that takes you from App to URL in one step.

![CI](https://github.com/epinio/epinio/workflows/CI/badge.svg)

<img src="./docs/epinio.png" width="50%" height="50%">

## Contents

- [Traefik](#traefik)
- [Linkerd](#linkerd)

## Traefik

When you installed Epinio, it looked at your cluster to see if you had
[Traefik](https://doc.traefik.io/traefik/providers/kubernetes-ingress/)
running. If it wasn't there it installed it.

As Epinio only check two namespaces for Traefik's presence, namely
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

## Linkerd

By default, Epinio installs [Linkerd](https://linkerd.io/) on your cluster. The various
namespaces created by Epinio become part of the Linkerd Service Mesh and thus
all communication between pods is secured with mTLS.

In some cases, you may not want Epinio to install Linkerd, either because you did that
manually before you install Epinio or other reason. You can provide the `--skip-linkerd`
flag to the `install` command to prevent Epinio from installing any of the Linkerd
control plane components:

```bash
$ epinio install --skip-linkerd
```
