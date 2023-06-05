# Setup external public registry
In this guide, we will demonstrate how to run a public container registry service using Docker. You will need a Linux computer with a public IP and FQDN domain assigned, as well as the `docker` runtime and a recent version of the `openssl` tool installed. You can find more details about setting up an SSL CA on the following reference: https://deliciousbrains.com/ssl-certificate-authority-for-local-https-development

**Disclaimer:** Use this guide at your own risk.

## Certificates
Before running the registry or the cluster we will need to create the keys and the certificates.

### Generate root CA key and certificate

Create and enter directory for your files:
```bash
mkdir certs
cd certs
```

Create root CA private key:
```bash
openssl genrsa -out CA.key 2048
```

Create root CA certificate:
```bash
openssl req -x509 -new -nodes \
    -subj "/C=DE/ST=Germany/L=Nurnberg/O=SUSE/OU=Epinio/CN=SUSE CA cert/emailAddress=epinio@suse.com" \
    -key CA.key \
    -sha256 \
    -days 3650 \
    -out CA.pem
```
### Create private key, CSR and signed certificate for external registry service

Create private key for your registry domain:
```bash
openssl genrsa -out registry.key 2048
```

Create a CSR request (CN value contains your registry domain):
```bash
openssl req -new \
    -subj "/C=DE/ST=Germany/L=Nurnberg/O=SUSE/OU=Epinio/CN=registry.suse.dev/emailAddress=epinio@suse.com" \
    -key registry.key \
    -out registry.csr
```

Create extra openssl config for with additional SAN entry (contains your registry domain):
```bash
cat > registry.ext <<EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = registry.suse.dev
EOF
```

Create signed registry certificate by using CSR, your CA and extra config:
```bash
openssl x509 -req -in registry.csr -CA CA.pem -CAkey CA.key \
    -CAcreateserial -out registry.pem -days 3650 -sha256 -extfile registry.ext
```

Verify registry certificate SAN entry:
```bash
openssl x509 -in registry.pem -text | grep -A1 'Subject Alternative Name'
>             X509v3 Subject Alternative Name: 
>                DNS:registry.suse.dev
```

## Setup the cluster
Transfer the CA.pem file to all kubernetes nodes and perform:
```bash
sudo cp CA.pem /etc/pki/trust/anchors/
sudo update-ca-certificates
sudo systemctl restart k3s[-agent].service
```

**Note:** This applies to openSUSE/SLE distributions and k3s. The method for distributing the CA cert may vary depending on the operating system and/or Kubernetes distribution being used.

## Setup the registry

Create a password `htpasswd` file for basic access auth on registry:
```bash
mkdir auth
docker run --entrypoint htpasswd httpd:2 -Bbn opensuse password > auth/htpasswd
```

Run the registry in docker:
```bash
docker run -d --restart=always --name=registry -v $PWD/auth:/auth -v $PWD:/certs \
    -e "REGISTRY_AUTH=htpasswd" \
    -e "REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm" \
    -e "REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd" \
    -e "REGISTRY_HTTP_ADDR=0.0.0.0:443" \
    -e "REGISTRY_HTTP_TLS_CERTIFICATE=/certs/registry.pem" \
    -e "REGISTRY_HTTP_TLS_KEY=/certs/registry.key" \
    -p 443:443 registry:2
```

**Note:** Check the certificate with:

```bash
curl --cacert CA.pem -u opensuse:password https://registry.suse.dev/v2/_catalog
```

### Install/Upgrade Epinio

Transfer the CA.pem file to a machine that has access to your cluster via the configured kubectl tool.

Create TLS secret for epinio registry:
```bash
kubectl create namespace epinio
kubectl create secret -n epinio generic epinio-external-registry-tls --from-file=tls.crt=/<PATH>/CA.pem
```

Install/Upgrade Epinio with external registry:
```bash
helm upgrade --install epinio --namespace epinio epinio/epinio \
    --set global.domain=mydomain.dev \
    --set global.registryURL=registry.suse.dev \
    --set global.registryUsername=opensuse \
    --set global.registryPassword=password \
    --set containerregistry.enabled=false \
    --set containerregistry.certificateSecret=epinio-external-registry-tls
```

### Test it

To test it we can push a sample application:

```
epinio app push -n sample -p assets/golang-sample-app
```

and we should see that the image were pushed into the external registry:

```bash
curl [--cacert CA.pem] -u opensuse:password https://registry.suse.dev/v2/_catalog
```