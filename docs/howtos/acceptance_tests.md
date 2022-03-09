# Acceptance Tests

## Basic Run

### Preparation

An accessible Kubernetes instance is required to run the acceptance tests.
If none is available, K3d can be
[setup locally](https://docs.epinio.io/installation/install_epinio_on_k3d.html).

The Kubernetes instance needs to be prepared prior to the test run. K3d has
its own make target for the preparation which can be initiated by running
`make prepare_environment_k3d`.

### Execution

Invoke `make test-acceptance` to running the epinio acceptance tests
with its standard configuration.

Invoke `make showfocus` to see if tests have been focused on.

This target is automatically run as part of `make test-acceptance` as
well.

## Configuration

The tests can be configured by a mixture of environment variables,
files, and, of course, by editing the test go files. The latter to
focus a run on specific tests, as per ginkgo's documentation.

### Environment variables

#### Required

   1. `KUBCONFIG`: This will give access the kubernetes cluster.
   
   2. `EPINIO_SETTINGS`: This will provide credentials to be used by tests
      to access the epinio server
   
   3. `EPINIO_BINARY`: This will provide the path of epinio binary to be
      used in the tests.

#### Optional

  1. `GINKGO_NODES`: The number of ginkgo nodes to distribute the
     tests across. The default is `2`. The CI flows use `8`.

  2. `FLAKE_ATTEMPTS`: The number of tries to perform when a test
     fails, to ensure that the failure is real, and not a flake,
     i.e. caused by a transient environmental condition. The default
     is `2`.

### Files

All file paths are specified relative to the top level directory of
the epinio checkout running the acceptance tests.

  1. `tmp/skip_cleanup`: If present system shutdown will not uninstall
     epinio from the tests cluster, nor will it tear down the test
     cluster. This allows future test runs to bypass the costly setup
     of a cluster and epinio installation.

     This cannot be avoided if changes are made to the (un)install
     code. This always requires a new epinio installation to test the
     latest changes.

     On the third hand, changing the server code still allows using
     this. In that case we simply cannot skip patching the server (See
     `EPINIO_SKIP_PATCH`), as that step updates it to the local code,
     and thus the last changes.

  1. `tmp/after_each_sleep`: If present, readable, and containing an
     integer number N the system will wait N seconds after each
     test. Enables the developer to inspect the cluster's state after
     a test.

     __Note__, the file has contain __only digits__, and nothing
     else. Even a trailing newline after the digits prevents the
     system from recognizing the request.
