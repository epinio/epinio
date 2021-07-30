# MS Azure AKS configuration

## Create an AKS cluster

If you don't have an existing cluster, follow the [quickstart](https://docs.microsoft.com/en-us/azure/aks/kubernetes-walkthrough) to create an AKS cluster.

## AKS Prerequisites

* Epinio v0.0.19 has been tested with Azure AKS incl. kubernetes versions v1.19.11 and v1.20.7
* Epinio Acceptance Tests passed on a 2 node cluster with Standard_D3_v2 instances
* To just try out Epinio, e.g. 2 Standard_D2_v2 are sufficient

#### Install Dependencies

Follow these [steps](./install_dependencies.md) to install dependencies.

#### Install Epinio CLI

* Download the binary

Find the latest version from [Releases](https://github.com/epinio/epinio/releases) and run e.g.

```bash
curl -o epinio -L https://github.com/epinio/epinio/releases/download/v0.0.19/epinio-linux-amd64
```

* Make the binary executable

```bash
chmod +x epinio
```

* Move the binary to your PATH

```bash
sudo mv ./epinio /usr/local/bin/epinio
```

#### Install

Installing Epinio in an Azure AKS cluster doesn't differ from the general installation documentation.
If you would just run `epinio install` it will automatically use a magic DNS system domain like e.g. `10.0.0.1.omg.howdoi.website`.

#### Install Ingress In Cluster (for a custom DOMAIN)

Install ingress first and wait for the `loadbalancer-ip` to be provisioned for the `traefik` ingress. Then, you can map the `loadbalancer-ip` to your `Domain Name` e.g. `example.com` and wait for it to be mapped.

```bash
epinio install-ingress
```

The output of the command will print the `loadbalancer-ip`.

#### Install Epinio In Cluster

```bash
epinio install --system-domain example.com --tls-issuer=letsencrypt-production --use-internal-registry-node-port=false
```

### Troubleshooting

With Epinio v0.0.19 `epinio install` can run into a timeout during linkerd deployment.
If that happened, you'll need to run `epinio uninstall`, and then install again.
The second installation should finish as expected.
