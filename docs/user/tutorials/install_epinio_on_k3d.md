# Creating a K3d kubernetes cluster

## Get K3d Kubernetes Cluster

### Install K3d

Follow the [instructions](https://k3d.io/) to install k3d on your system. 

#### Create a K3d kubernetes cluster

```bash
$ k3d cluster create epinio
```

#### Create a K3d kubernetes cluster when it is inside a VM

Epinio has to connect to pods inside the cluster. The default installation uses the internal docker IP for this. If docker is running in a VM, e.g. with Docker Desktop for Mac, that IP will not be reachable.

As a workaround the IP of the host can be used instead, together with port-forwardings:

```bash
k3d cluster create epinio -p '80:80@server[0]' -p '443:443@server[0]'
```

After the command returns, `kubectl` should already be talking to your new cluster:

```bash
$ kubectl get nodes
NAME                  STATUS   ROLES                  AGE   VERSION
k3d-epinio-server-0   Ready    control-plane,master   38s   v1.20.0+k3s2
```

#### Install Dependencies 

Follow these [steps](./install_dependencies.md) to install dependencies.

#### Install Epinio CLI

##### Linux

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

##### MacOS 

* Download the binary

```bash
curl -o epinio -L https://github.com/epinio/epinio/releases/download/v0.0.18/epinio-darwin-amd64
```

* Make the binary executable

```bash
chmod +x epinio
```

* Move the binary to your PATH

```bash
sudo mv ./epinio /usr/local/bin/epinio
```

##### Windows 

```bash
 curl -LO https://github.com/epinio/epinio/releases/download/v0.0.18/epinio-windows-amd64
```

#### Install Epinio in cluster

```bash
epinio install
```

#### Install Epinio in cluster when K3d is inside a VM

```bash
epinio install --system-domain=<YOUR-IP>.omg.howdoi.website
```

`<YOUR-IP>` can be found by running 

```bash
ifconfig |grep "inet.*broadcast
```
 
### Troubleshooting 

In case of trouble with Epinio's Traefik component or Ingress controllers, the [Traefik](../explanations/advanced.md#traefik) section in the
[Advanced Topics](../explanations/advanced.md) document shall be your friend.