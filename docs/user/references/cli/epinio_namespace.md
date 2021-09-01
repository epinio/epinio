---
title: "epinio namespace"
linkTitle: "epinio namespace"
weight: 1
---
## epinio namespace

Epinio-controlled namespaces

### Synopsis

Manage epinio-controlled namespaces

```
epinio namespace [flags]
```

### Options

```
  -h, --help   help for namespace
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
* [epinio namespace create](../epinio_namespace_create)	 - Creates an epinio-controlled namespace
* [epinio namespace delete](../epinio_namespace_delete)	 - Deletes an epinio-controlled namespace
* [epinio namespace list](../epinio_namespace_list)	 - Lists all epinio-controlled namespaces

