# Creating a K3d kubernetes cluster

## Create a GKE cluster

Follow the [quickstart](https://cloud.google.com/kubernetes-engine/docs/quickstart) to create a GKE cluster.

#### Install Dependencies 

Follow these [steps](./install_dependencies.md) to install dependencies.

#### Install Epinio CLI

* Download the binary

```bash
curl -o epinio -L https://github.com/epinio/epinio/releases/download/v0.0.18/epinio-linux-amd64
```

* Make the binary executable

```bash
chmod +x epinio
```

* Move the binary to your PATH

```bash
sudo mv ./epinio /usr/local/bin/epinio
```

#### Install Ingress in cluster

In GKE, we install ingress first and wait for the `loadbalancer-ip` to be provisioned by GKE for the `traefik` ingress. Then, you can map the `loadbalancer-ip` to your `Domain Name` e.x `example.com` and wait for it to be mapped.

```bash
epinio install-ingress
```

The output of the command will print the `loadbalancer-ip`.

#### Install Epinio in cluster

```bash
epinio install --system-domain example.com
```

### Troubleshooting 

In case of trouble with Epinio's Traefik component or Ingress controllers, the [Traefik](../explanations/advanced.md#traefik) section in the [Advanced Topics](../explanations/advanced.md) document shall be your friend.