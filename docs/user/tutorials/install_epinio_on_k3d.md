# Creating a K3d Kubernetes Cluster

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

### Install Epinio on the Cluster

Follow [Installation using a MagicDNS Service](./install_epinio_magicDNS.md) to install Epinio in your test environment.

If k3d is inside a VM, in addition to the special k3d setup, explained above, use this system domain instead:

```bash
epinio install --system-domain=<YOUR-IP>.omg.howdoi.website
```

`<YOUR-IP>` can be found by running

```bash
ifconfig |grep "inet.*broadcast
```


### Troubleshooting

#### Kubeconfig

To get the kube config to access the cluster:

```
k3d kubeconfig get epinio
```

#### Traefik

In case of trouble with Epinio's Traefik component or Ingress controllers, the [Traefik](../explanations/advanced.md#traefik) section in the
[Advanced Topics](../explanations/advanced.md) document shall be your friend.
