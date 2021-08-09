---
title: "epinio push"
linkTitle: "epinio push"
weight: 1
---
## epinio push

Push an application from the specified directory, or the current working directory

```
epinio push NAME [URL|PATH_TO_APPLICATION_SOURCES] [flags]
```

### Options

```
  -b, --bind strings              services to bind immediately
      --builder-image string      paketo builder image to use for staging (default "paketobuildpacks/builder:full")
      --docker-image-url string   docker image url for the app workload image
      --git string                git revision of sources. PATH becomes repository location
  -h, --help                      help for push
  -i, --instances int32           The number of desired instances for the application, default only applies to new deployments (default 1)
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

