# Install Epinio using a Magic DNS Service

This is about just running `epinio install`. It should work on nearly any kubernetes distribution and provides you with a test environment.
You will automatically get a magic wildcard domain like "10.0.0.1.omg.howdoi.website" which points to the public IP of Traefik.

## Install the Epinio CLI

If not done already, refer to [Install the Epinio CLI](./docs/user/tutorials/install_epinio_cli.md).

### Install Epinio on the Cluster

```bash
epinio install
```

### Troubleshooting

#### Traefik

In case of trouble with Epinio's Traefik component or Ingress controllers, the [Traefik](../explanations/advanced.md#traefik) section in the
[Advanced Topics](../explanations/advanced.md) document shall be your friend.

#### DNS Rebind Protection

Some routers filter queries where the answer consists of IP addresses from the private range, like "10.0.0.1".

This stops a malicous website from probing the local network for hosts.

Amongst those routers is the AVM FRITZBox and everything that uses [dnsmasq](https://thekelleys.org.uk/dnsmasq/docs/dnsmasq-man.html) with `stop-dns-rebind`, like [pfSense](https://docs.netgate.com/pfsense/en/latest/services/dns/rebinding.html) or NetworkManager.

If you still want to use the default magic DNS, you'll have to whitelist `omg.howdoi.website` in your local DNS server.
