---
title: "epinio install-cert-manager"
linkTitle: "epinio install-cert-manager"
weight: 1
---
## epinio install-cert-manager

install Epinio's cert-manager in your configured kubernetes cluster

### Synopsis

install Epinio cert-manager controller in your configured kubernetes cluster

```
epinio install-cert-manager [flags]
```

### Options

```
      --email-address string   The email address you are planning to use for getting notifications about your certificates (default "epinio@suse.com")
  -h, --help                   help for install-cert-manager
  -i, --interactive            Whether to ask the user or not (default not)
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

