---
title: "epinio service"
linkTitle: "epinio service"
weight: 1
---
## epinio service

Epinio service features

### Synopsis

Handle service features with Epinio

### Options

```
  -h, --help   help for service
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
* [epinio service bind](../epinio_service_bind)	 - Bind a service to an application
* [epinio service create](../epinio_service_create)	 - Create a service
* [epinio service create-custom](../epinio_service_create-custom)	 - Create a custom service
* [epinio service delete](../epinio_service_delete)	 - Delete a service
* [epinio service list](../epinio_service_list)	 - Lists all services
* [epinio service list-classes](../epinio_service_list-classes)	 - Lists the available service classes
* [epinio service list-plans](../epinio_service_list-plans)	 - Lists all plans provided by the named service class
* [epinio service show](../epinio_service_show)	 - Service information
* [epinio service unbind](../epinio_service_unbind)	 - Unbind service from an application

