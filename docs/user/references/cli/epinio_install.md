---
title: "epinio install"
linkTitle: "epinio install"
weight: 1
---
## epinio install

install Epinio in your configured kubernetes cluster

### Synopsis

install Epinio PaaS in your configured kubernetes cluster

```
epinio install [flags]
```

### Options

```
      --email-address string                 The email address you are planning to use for getting notifications about your certificates (default "epinio@suse.com")
      --enable-internal-registry-node-port   Make the internal registry accessible via a node port, so kubelet can access the registry without trusting its cert. (default true)
  -h, --help                                 help for install
  -i, --interactive                          Whether to ask the user or not (default not)
      --password string                      The password for authenticating all API requests
  -s, --skip-default-org                     Set this to skip creating a default org
      --skip-linkerd                         Assert to epinio that Linkerd is already installed.
      --skip-traefik                         Assert to epinio that there is a Traefik active, even if epinio cannot find it.
      --system-domain string                 The domain you are planning to use for Epinio. Should be pointing to the traefik public IP (Leave empty to use a omg.howdoi.website domain).
      --tls-issuer string                    The name of the cluster issuer to use. Epinio creates three options: 'epinio-ca', 'letsencrypt-production', and 'selfsigned-issuer'. (default "epinio-ca")
      --user string                          The user name for authenticating all API requests
```

### Options inherited from parent commands

```
      --config-file string       (EPINIO_CONFIG) set path of configuration file (default "~/.config/epinio/config.yaml")
  -c, --kubeconfig string        (KUBECONFIG) path to a kubeconfig, not required in-cluster
      --no-colors                Suppress colorized output
      --skip-ssl-verification    (SKIP_SSL_VERIFICATION) Skip the verification of TLS certificates
      --timeout-multiplier int   (EPINIO_TIMEOUT_MULTIPLIER) Multiply timeouts by this factor (default 1)
      --trace-level int          (TRACE_LEVEL) Only print trace messages at or above this level (0 to 5, default 0, print nothing)
      --verbosity int            (VERBOSITY) Only print progress messages at or above this level (0 or 1, default 0)
```

### SEE ALSO

* [epinio](../epinio)	 - Epinio cli

