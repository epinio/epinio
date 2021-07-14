---
title: "epinio config"
linkTitle: "epinio config"
weight: 1
---
## epinio config

Epinio config management

### Synopsis

Manage the epinio cli configuration

### Options

```
  -h, --help   help for config
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
* [epinio config colors](../epinio_config_colors)	 - Manage colored output
* [epinio config show](../epinio_config_show)	 - Show the current configuration
* [epinio config update-credentials](../epinio_config_update-credentials)	 - Update the stored credentials

