# Using a Certificate Issuer

Epinio comes with multiple [cert-manager cluster issuers](https://cert-manager.io/docs/configuration/) for creating certificates:

* epinio-ca (default)
* letsencrypt-production
* selfsigned-issuer

The issuer will be used for both, the Epinio API endpoint and workloads (i.e. pushed applications).

## Choosing a Different Issuer

When installing Epinio, one can choose between those issuers by using the `--tls-issuer` argument:

```
epinio install --tls-issuer=letsencrypt-production
```

## Using a custom issuer

It's possible to create a cert-manager cluster issuer in the cluster, before installing Epinio and referencing it by name when installing.

*Note*: The cert-manager cluster-issuer CRD must be applied to the cluster.

### ACME DNS Challenge

For example to use Letsencrypt with a DNS challenge, which supports wildcards and private IPs:

```
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: dns-staging
spec:
  acme:
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    privateKeySecretRef:
      name: example-issuer-account-key
    solvers:
    - dns01:
        cloudflare:
          email: user@example.com
          apiKeySecretRef:
            name: cloudflare-apikey-secret
            key: apikey
      selector:
        dnsNames:
        - 'example.com'
        - '*.example.com'
```

Note: This uses the staging endpoint for testing.

You can then install Epinio:

```
epinio install --tls-issuer=dns-staging
```

## Use TLS When Pulling From Internal Registry

Epinio comes with its own registry for Docker images. This registry needs to be reachable from the Kubernetes nodes.
If you are using a certificate issuer whose CA is trusted by the Kubernetes nodes, you can turn on SSL for pulling images from the internal registry:

```
epinio install --tls-issuer=letsencrypt-production --use-internal-registry-node-port=false
```

Without the node port, pushing images to the registry uses the "epinio-registry" ingress, which is handled by Traefik.

# Background on Cert Manager and Issuers

Cert manager watches for a *certificate* resource and uses the referenced *cluster issuer* to generate a certificate.
The certificate is stored in a *secret*, in the namespace the certificate resources was created in.
An *ingress* resource can then use that secret to set up TLS.

Example:

1. `epinio install` creates a certificate resource in epinio namespace

```
apiVersion: cert-manager.io/v1alpha2
kind: Certificate
metadata:
  name: epinio
  namespace: epinio
spec:
  commonName: epinio.172.27.0.2.omg.howdoi.website
  dnsNames:
  - epinio.172.27.0.2.omg.howdoi.website
  issuerRef:
    kind: ClusterIssuer
    name: epinio-ca
  secretName: epinio-tls
```

2. cert-manager creates the 'epinio-tls' secret, using the referenced cluster issuer 'epinio-ca'

```
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  annotations:
    cert-manager.io/alt-names: epinio.172.27.0.2.omg.howdoi.website
    cert-manager.io/certificate-name: epinio
    cert-manager.io/common-name: epinio.172.27.0.2.omg.howdoi.website
    cert-manager.io/ip-sans: ""
    cert-manager.io/issuer-group: ""
    cert-manager.io/issuer-kind: ClusterIssuer
    cert-manager.io/issuer-name: epinio-ca
    cert-manager.io/uri-sans: ""
  name: epinio-tls
  namespace: epinio
data:
  ca.crt: ...
  tls.crt: ...
  tls.key: ...
```

3. Epinio creates an ingress resource

```
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    kubernetes.io/ingress.class: traefik
    traefik.ingress.kubernetes.io/router.entrypoints: websecure
    traefik.ingress.kubernetes.io/router.middlewares: epinio-epinio-api-auth@kubernetescrd
    traefik.ingress.kubernetes.io/router.tls: "true"
  labels:
    app.kubernetes.io/name: epinio
  name: epinio
  namespace: epinio
spec:
  rules:
  - host: epinio.172.27.0.2.omg.howdoi.website
    http:
      paths:
      - backend:
          service:
            name: epinio-server
            port:
              number: 80
        path: /
        pathType: ImplementationSpecific
  tls:
  - hosts:
    - epinio.172.27.0.2.omg.howdoi.website
    secretName: epinio-tls
```

## Epinio Push

The same is true for applications, `epinio push` creates a `certificate` in the app's workspace and cert-manager creates a secret for the app's ingress.
