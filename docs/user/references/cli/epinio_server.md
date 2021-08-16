---
title: "epinio server"
linkTitle: "epinio server"
weight: 1
---
## epinio server

Starts the Epinio server.

### Synopsis

This command starts the Epinio server. `epinio install` ensures the server is running inside your cluster. Normally you don't need to run this command manually.

```
epinio server [flags]
```

### Options

```
  -h, --help                              help for server
      --port int                          (PORT) The port to listen on. Leave empty to auto-assign a random port
      --tls-issuer string                 (TLS_ISSUER) The cluster issuer to use for workload certificates (default "epinio-ca")
      --use-internal-registry-node-port   (USE_INTERNAL_REGISTRY_NODE_PORT) Use the internal registry via a node port (default true)
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

