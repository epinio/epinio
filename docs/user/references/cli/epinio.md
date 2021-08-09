---
title: "epinio"
linkTitle: "epinio"
weight: 1
---
## epinio

Epinio cli

### Synopsis

epinio cli is the official command line interface for Epinio PaaS 

### Options

```
      --config-file string       (EPINIO_CONFIG) set path of configuration file (default "~/.config/epinio/config.yaml")
  -h, --help                     help for epinio
  -c, --kubeconfig string        (KUBECONFIG) path to a kubeconfig, not required in-cluster
      --no-colors                Suppress colorized output
      --skip-ssl-verification    (SKIP_SSL_VERIFICATION) Skip the verification of TLS certificates
      --timeout-multiplier int   (EPINIO_TIMEOUT_MULTIPLIER) Multiply timeouts by this factor (default 1)
      --trace-level int          (TRACE_LEVEL) Only print trace messages at or above this level (0 to 5, default 0, print nothing)
      --verbosity int            (VERBOSITY) Only print progress messages at or above this level (0 or 1, default 0)
```

### SEE ALSO

* [epinio app](../epinio_app)	 - Epinio application features
* [epinio completion](../epinio_completion)	 - Generate completion script for a shell
* [epinio config](../epinio_config)	 - Epinio config management
* [epinio disable](../epinio_disable)	 - disable Epinio features
* [epinio enable](../epinio_enable)	 - enable Epinio features
* [epinio info](../epinio_info)	 - Shows information about the Epinio environment
* [epinio install](../epinio_install)	 - install Epinio in your configured kubernetes cluster
* [epinio install-cert-manager](../epinio_install-cert-manager)	 - install Epinio's cert-manager in your configured kubernetes cluster
* [epinio install-ingress](../epinio_install-ingress)	 - install Epinio's Ingress in your configured kubernetes cluster
* [epinio org](../epinio_org)	 - Epinio organizations
* [epinio push](../epinio_push)	 - Push an application from the specified directory, or the current working directory
* [epinio server](../epinio_server)	 - starts the Epinio server. You can connect to it using either your browser or the Epinio client.
* [epinio service](../epinio_service)	 - Epinio service features
* [epinio target](../epinio_target)	 - Targets an organization in Epinio.
* [epinio uninstall](../epinio_uninstall)	 - uninstall Epinio from your configured kubernetes cluster
* [epinio version](../epinio_version)	 - Print the version number

