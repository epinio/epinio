# Setup a local external registry

## Certificates

Before running the registry or the cluster we will need to create the certificates using this `epinio.cfg` configuration file, with a wildcard SAN:

```
[ req ]
default_bits       = 2048
distinguished_name = req_distinguished_name
req_extensions     = req_ext
[ req_distinguished_name ]
countryName                = Country Name (2 letter code)
stateOrProvinceName        = State or Province Name (full name)
localityName               = Locality Name (eg, city)
organizationName           = Organization Name (eg, company)
commonName                 = Common Name (e.g. server FQDN or YOUR name)
[ req_ext ]
subjectAltName = @alt_names
[alt_names]
DNS.1   = *.nip.io
```


We are going to put them in a `docker_reg_certs` folder:

```
mkdir docker_reg_certs
```

and we can now run these commands:

```
# create the Certificate Signing Request
openssl req -new -newkey rsa:4096 -nodes \
    -config epinio.cfg \
    -subj "/C=DE/ST=Germany/L=Nuremberg/O=SUSE/OU=Epinio" \
    -keyout docker_reg_certs/epinio.key \
    -out docker_reg_certs/epinio.csr

# create the certificate
openssl x509 -req -sha256 -days 365 \
    -extfile <(printf "subjectAltName=DNS:*.nip.io") \
    -in docker_reg_certs/epinio.csr \
    -signkey docker_reg_certs/epinio.key \
    -out docker_reg_certs/epinio.pem
```

**Note:** Check the SAN of request and certificate by running:

```
openssl req -in docker_reg_certs/epinio.csr -text | grep -A1 'Subject Alternative Name'
openssl x509 -in docker_reg_certs/epinio.pem -text  | grep -A1 'Subject Alternative Name'
```

## Setup the cluster

With `k3d` we'll need to mount the certificate during the cluster creation, so you will need to mount it adding this flag `--volume $(pwd)/docker_reg_certs/epinio.pem:/etc/ssl/certs/epinio.pem` (see [k3d doc](https://k3d.io/v5.2.1/faq/faq/#pods-fail-to-start-x509-certificate-signed-by-unknown-authority).
)).

If this is not done the kubelet will not be able to pull the created images.

We can provide this additional flag exporting it to the `EPINIO_K3D_INSTALL_ARGS` environment variable:

```
export EPINIO_K3D_INSTALL_ARGS="--volume $(pwd)/docker_reg_certs/epinio.pem:/etc/ssl/certs/epinio.pem"
```

and we can now create the cluster with the `make acceptance-cluster-setup` command.


## Epinio setup

**Note:** with `k3d` we can get and merge the kubeconfig with `k3d kubeconfig merge -ad`, and then we can switch the context with `kubectl config use-context k3d-epinio-acceptance`.

```
k3d kubeconfig merge -ad && kubectl config use-context k3d-epinio-acceptance
```

Install cert-manager and Epinio:

```
make install-cert-manager && make prepare_environment_k3d
```

## Setup the registry

We can run the registry, attaching it to the same k3d Epinio network:

```
docker run -d -p 5000:5000 --name registry --rm \
    --network epinio-acceptance \
    -v $PWD/docker_reg_certs:/certs \
    -v /reg:/var/lib/registry \
    -e REGISTRY_HTTP_TLS_CERTIFICATE=/certs/epinio.pem \
    -e REGISTRY_HTTP_TLS_KEY=/certs/epinio.key registry:2
```

and we can find the IP and URL of the registry with:

```
REGISTRY_IP=$(docker inspect registry | jq -r ".[].NetworkSettings.Networks[\"epinio-acceptance\"].IPAddress")
REGISTRY_URL=$(printf "%s.nip.io:5000" $(echo $REGISTRY_IP | sed 's/\./-/g')) && echo $REGISTRY_URL
```

**Note:** Check the certificate with:

```
curl -L --cacert docker_reg_certs/epinio.pem https://$REGISTRY_URL/v2
```

### Upgrade Epinio

We can now upgrade Epinio to use this external registry:

```
./scripts/prepare-environment-k3d.sh \
    --set containerregistry.enabled=false \
    --set global.registryURL=$REGISTRY_URL \
    --set global.registryNamespace=docker-local \
    --set global.registryUsername=admin \
    --set global.registryPassword=password
```

To check the values:
```
helm get values -n epinio epinio
```

To make Epinio work we need to mount this certificate into the staging job.  

To do so create a secret with the certificate, and patch the Epinio deployment to fetch this secret:

```
kubectl create secret -n epinio generic epinio-external-registry-tls --from-file=tls.crt=docker_reg_certs/epinio.pem

kubectl patch deployment -n epinio epinio-server --patch '{"spec": {"template": {"spec": {"containers": [{"name": "epinio-server","env": [{"name":"REGISTRY_CERTIFICATE_SECRET", "value":"epinio-external-registry-tls"}]}]}}}}'
```

Now the certificate will be trusted by the staging job.

### Test it

To test it we can push a sample application:

```
epinio app push -n sample -p assets/golang-sample-app
```

and we should see that the image were pushed into the external registry:

```
curl -L --cacert docker_reg_certs/epinio.pem https://$REGISTRY_URL/v2/_catalog
```
