# Setup a local external registry

## Certificates

Before running the registry or the cluster we will need to create the certificates.

We are going to put them in a `docker_reg_certs` folder:

```
mkdir docker_reg_certs
```

Using this `epinio.cfg` configuration file

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
DNS.1   = omg.howdoi.website
DNS.2   = *.omg.howdoi.website
DNS.3   = 172.21.0.5.omg.howdoi.website
```

we can now run these commands:

```
# create the Certificate Signing Request
openssl req -new -newkey rsa:4096 -nodes \
    -config epinio.cfg \
    -subj "/C=IT/ST=Italy/L=Rome/O=SUSE/OU=Epinio/CN=*.omg.howdoi.website" \
    -keyout docker_reg_certs/epinio.key \
    -out docker_reg_certs/epinio.csr

# create the certificate
openssl x509 -req -sha256 -days 365 \
    -extfile <(printf "subjectAltName=DNS:172.21.0.5.omg.howdoi.website,DNS:*.omg.howdoi.website") \
    -in docker_reg_certs/epinio.csr \
    -signkey docker_reg_certs/epinio.key \
    -out docker_reg_certs/epinio.pem
```

**Note:** to check the SAN of the request and the certificate you can run:

```
openssl req -in docker_reg_certs/epinio.csr -text | grep -A1 'Subject Alternative Name'

openssl x509 -in docker_reg_certs/epinio.pem -text  | grep -A1 'Subject Alternative Name'
```

## Setup the cluster

With `k3d` we'll need to mount the certificate during the cluster creation, so you will need to mount it adding this flag `--volume $(pwd)/docker_reg_certs/epinio.pem:/etc/ssl/certs/epinio.pem`:

i.e.:
```
k3d cluster create --volume /path/to/your/certs.crt:/etc/ssl/certs/yourcert.crt
```

We need to do that otherwise the kubelet will not be able to pull the created images.

See [k3d doc](https://k3d.io/v5.2.1/faq/faq/#pods-fail-to-start-x509-certificate-signed-by-unknown-authority).


## Setup the registry

We can now run our registry:

```
docker run -d -p 5000:5000 --name registry --rm \
    -v $PWD/docker_reg_certs:/certs \
    -v /reg:/var/lib/registry \
    -e REGISTRY_HTTP_TLS_CERTIFICATE=/certs/epinio.pem \
    -e REGISTRY_HTTP_TLS_KEY=/certs/epinio.key registry:2
```

Now the registry needs to be accessible from the cluster. We can do that adding it to the same docker network, and check the IP that it got:

```
docker network connect epinio-acceptance registry
docker inspect registry | jq -r ".[].NetworkSettings.Networks[\"epinio-acceptance\"].IPAddress"
```

**Note:** to check the certificate you can curl with:

```
curl -L --cacert docker_reg_certs/epinio.pem https://172.21.0.5.omg.howdoi.website:5000/v2
```

## Epinio setup

We can now install/upgrade Epinio setting up an external registry with the proper configuration:

```
helm upgrade --install epinio -n epinio --create-namespace helm-charts/chart/epinio \
    --set global.domain=172.21.0.4.omg.howdoi.website \
    --set containerregistry.enabled=false \
    --set global.registryURL=172.21.0.5.omg.howdoi.website:5000 \
    --set global.registryNamespace=docker-local \
    --set global.registryUsername=admin \
    --set global.registryPassword=password \
    --set server.disableTracking=true
```

To make Epinio works we need to mount this certificate during the staging job.  

To do so let's create a secret with the certificate, and patch the Epinio deployment to fetch this secret:

```
kubectl create secret -n epinio generic epinio-external-registry-tls --from-file=tls.crt=docker_reg_certs/epinio.pem

kubectl patch deployment -n epinio epinio-server --patch '{"spec": {"template": {"spec": {"containers": [{"name": "epinio-server","env": [{"name":"REGISTRY_CERTIFICATE_SECRET", "value":"epinio-external-registry-tls"}]}]}}}}'
```

Now the certificate will be trusted by the staging job.
