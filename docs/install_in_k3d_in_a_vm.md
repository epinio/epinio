### k3d inside a VM

Epinio has to connect to pods inside the cluster. The default installation uses the internal docker IP for this. If docker is running in a VM, e.g. with Docker Desktop for Mac, that IP will not be reachable.
As a workaround the IP of the host can be used instead, together with port-forwardings:

```bash
k3d cluster create epinio -p '80:80@server[0]' -p '443:443@server[0]'
epinio install --system-domain=<YOUR-IP>.omg.howdoi.website
```

The host's interface IP can often be found, depending on the machine's network setup, by running: `ifconfig |grep "inet.*broadcast"`

More information can be found in the [detailed push process docs](docs/detailed-push-process.md#6-additional-things).