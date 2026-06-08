# Epinio with Gateway API

Gateway API is now supported by Epinio as an alternative to Ingress that also coincides with the EOL of Ingress NGINX.  If you are asking yourself why you should make the switch, we will refer you to official Kubernetes documentation on [Reasons to Switch to Gateway API](https://gateway-api.sigs.k8s.io/guides/getting-started/migrating-from-ingress/#reasons-to-switch-to-gateway-api).

## Prerequisites

### Gateway Controller

Before configuring Epinio to leverage Gateway API, you need a **Gateway Controller**.  There are several options to consider, to name a couple:

- [Traefik Gateway API](https://doc.traefik.io/traefik/reference/install-configuration/providers/kubernetes/kubernetes-gateway/)
- [Cilium Gateway API (w/ Envoy)](https://docs.cilium.io/en/latest/network/servicemesh/gateway-api/gateway-api/)

You will be able to install one or several options with **Helm**, supplying values to opt-in to their Gateway API support.  Once you have followed and completed their setup instructions, you will then be able to leverage pertinent **Custom Resource Definitions**, for example: `Gateway`, `HTTPRoute`, and `TCPRoute`.  **Epinio** simplifies the deployment of these custom resource definitions via the chart's values interface.


### Knowledge of Epinio Installation

For simplification purposes of this walkthrough, we assume that you know how to install and configure Epinio already.  If you do not already have a firm grasp on this process, please refer to our existing [documentation](https://docs.epinio.io/installation/install_epinio).


## Setup Gateway API

For walkthrough purposes, we'll utilize **Traefik**.

1. Install Gateway API CRDs

```bash
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.5.1/standard-install.yaml
```

2. Install Traefik RBAC for Gateway API

```bash
kubectl apply -f https://raw.githubusercontent.com/traefik/traefik/v3.7/docs/content/reference/dynamic-configuration/kubernetes-gateway-rbac.yml
```

3. Install **Traefik** with Gateway API configs:

```bash
helm repo add traefik https://traefik.github.io/charts
helm repo update
helm install traefik traefik/traefik \
  --namespace traefik \
  --create-namespace \
  --set providers.kubernetesGateway.enabled=true
```

We will now have the necessary `GatewayClass` CRD deployed to our cluster.


## Install Epinio's Gateway API Resources

Update your Helm values to leverage Epinio's Gateway API resources, simply:

- `Gateway`

```yaml
## --set gateway.enabled=true
gateway:
  enabled: true
```

- `HTTPRoute`

```yaml
## --set httpRoute.enabled=true
httpRoute:
  enabled: true
```

- Domain name

```yaml
## --set global.domain=127.0.0.1.sslip.io
global:
  domain: 127.0.0.1.sslip.io
```

❗**IMPORTANT**:

We allow the existence of both Ingress & Gateway API resources to facilitate migration efforts in dev/qa.  **However**, we advise that you practice one implementation in production.  If you have enabled your Gateway API resources, disable Ingress resources by setting:

```yaml
## --set ingress.enabled=false
ingress:
  enabled: false
```

Once these Helm values have been applied to your installation via `helm upgrade --install`, your `Gateway` and `HTTPRoute` resources will be deployed to the cluster, ready to handle incoming traffic.


### Additional Configuration

There are additional configurations to control the behavior for Epinio's usage of Gateway API.

- `gateway.hostnameOverride` & `gateway.dexHostnameOverride`
    - Configurable values to override the defaults determined by `global.domain`
- `gateway.gatewayClassName`
    - Determines the Gateway Controller's class that we wish to use.  For this walkthrough, we installed Traefik, thus our class name is `traefik` which happens to be the default.
- `gateway.tls.enabled`
    - Determines whether or not we secure traffic to Epinio with an HTTPS redirect within Kubernetes at the Gateway.  In order to enable an HTTPS redirect, set to `true`.
- `gateway.annotations` & `httpRoute.annotations`
    - Provide any annotations necessary for your custom implementations, such as certificate issuers.

There are more configurations available however these are the most relevant and anticipated for customization.


## Verify Functionality

You should be able to visit your specified `.Values.global.domain` value and reach Epinio.  If not, there are individual items to troubleshoot:

1. Does my **Gateway Controller's** `LoadBalancer` service have an **External IP**?  Verify a value is provided in the `External-IP` column.

```bash
kubectl get svc
```

2. Can I reach the **External IP** of the **Gateway Controller's** `Loadbalancer` service?

3. Is my domain configured properly via **Epinio** values?  Is DNS configured appropriately for the domain and IP address?

