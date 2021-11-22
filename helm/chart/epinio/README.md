# Epinio Helm Chart

From app to URL in one command.

## Introduction

This helm chart can be used to deploy Epinio on a cluster. It is an alternative
to `epinio install` command.

## Prerequisites

Epinio needs some external components to be running on your cluster in order to
work. You may already have those deployed, otherwise follow the instructions here
to deploy them.

Important: Some of the namespaces of the components are hardcoded in the Epinio
code and thus are important to be the same as describe here. In the future this
may be configurable on the Epinio Helm chart.

### Linkerd

- Optional

Download the linkerd cli from here: https://github.com/linkerd/linkerd2/releases/tag/stable-2.10.2

Install linkerd with:

```
$ kubectl create namespace linkerd
$ kubectl apply -f assets/embedded-files/linkerd/rbac.yaml 
$ linkerd install | kubectl apply -f - && linkerd check --wait 10m
```

### Traefik

Install Traefik with:

```
$ kubectl create namespace traefik
$ export LOAD_BALANCER_IP=$(LOAD_BALANCER_IP:-) # Set this to the IP of your load balancer if you know that
$ helm install traefik --namespace traefik "https://helm.traefik.io/traefik/traefik-10.3.4.tgz" \
		--set globalArguments='' \
		--set-string deployment.podAnnotations.linkerd\\.io/inject=enabled \
		--set-string ports.web.redirectTo=websecure \
		--set-string ingressClass.enabled=true \
		--set-string ingressClass.isDefaultClass=true \
		--set-string service.spec.loadBalancerIP=$LOAD_BALANCER_IP
```

### Install Kubed

```
$ kubectl create namespace kubed 
$ helm repo add appscode https://charts.appscode.com/stable/
$ helm repo update
$ helm install kubed --namespace kubed --version v0.12.0 appscode/kubed
```

### Install Cert Manager

```
$ kubectl create namespace cert-manager
$ helm repo add jetstack https://charts.jetstack.io
$ helm repo update
$ helm install cert-manager --namespace cert-manager jetstack/cert-manager \
		--set installCRDs=true \
		--set extraArgs[0]=--enable-certificate-owner-ref=true
```

### Install Tekton

```
$ kubectl create namespace tekton
$ kubectl label namespace tekton "linkerd.io/inject"="enabled"
$ kubectl apply -f https://raw.githubusercontent.com/epinio/epinio/main/assets/embedded-files/tekton/pipeline-v0.28.0.yaml
```

### Install Minio (Optional)

Any S3 compatible storage can be used

TODO: Describe a deployment of Minio here

### Install Registry (Optional)

Any container registry that supports basic auth authentication can be used (e.g. gcr, dockerhub etc)

TODO: Describe a deployment of a registry here

### Install Epinio

```
$ kubectl create namespace epinio
$ kubectl label namespace epinio "linkerd.io/inject"="enabled"
```

Create a `values.yaml` file for Epinio. Look at the available options here:
https://github.com/epinio/epinio/blob/documentation-installer/helm/chart/epinio/values.yaml

Install Epinio with:

```
$ helm install epinio helm/chart/epinio/ -n epinio --values values.yaml
```
