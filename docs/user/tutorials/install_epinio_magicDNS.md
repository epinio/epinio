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
