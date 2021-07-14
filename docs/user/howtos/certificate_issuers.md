# Using a Certificate Issuer

Epinio comes with two [cert-manager issuers](https://cert-manager.io/docs/configuration/) for creating certificates:

* epinio-ca (default)
* letsencrypt-production
* selfsigned-issuer

The issuer will be used for both, the Epinio API endpoint and workloads (i.e. pushed applications).

## Choosing a Different Issuer

During installation of Epinio, one can switch between those issuers by using the `--tls-issuer` argument:

```
epinio install --tls-issuer=letsencrypt-production
```

## Using a custom issuer

It's possible to create a cert-manager cluster issuer in the cluster, before installing Epinio and referencing it by name when installing.

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

Pushing images to the registry uses the "epinio-registry" ingress otherwise, where encryption is handled by Traefik.
