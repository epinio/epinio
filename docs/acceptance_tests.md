# Acceptance Tests

## Basic Run

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

  1. `GINKGO_NODES`: The number of ginkgo nodes to distribute the
     tests across. The default is `2`. The CI flows use `8`.

  1. `FLAKE_ATTEMPTS`: The number of tries to perform when a test
     fails, to ensure that the failure is real, and not a flake,
     i.e. caused by a transient environmental condition. The default
     is `2`.

  1. `EPINIO_SKIP_PATCH`: When present and not empty the test startup
     will skip over patching the epinio server pod of the test
     cluster. This should be used when the test cluster still has a an
     epinio installation from a previous run. See `tmp/skip_cleanup`
     as well.

  1. `EPINIO_K3D_INSTALL_ARGS`: When present and not empty the content
     is passed to the `k3d` command used to create a new test
     cluster. This can be used to adapt the cluster to nonstandard
     environments.

     For example `-p '80:80@server[0]' -p '443:443@server[0]'` would
     set up specific port mappings.

### Files

All file paths are specified relative to the top level directory of
the epinio checkout running the acceptance tests.

  1. `tmp/skip_cleanup`: If present system shutdown will not uninstall
     epinio from the tests cluster, nor will it tear down the test
     cluster. This allows future test runs to bypass the costly setup
     of a cluster and epinio installation.

     Note however that this should not be done when testing changed to
     epinio's server side. This always requires a new epinio
     installation to test the latest changes.

  1. `tmp/after_each_sleep`: If present, readable, and containing an
     integer number N the system will wait N seconds after each
     test. Enables the developer to inspect the cluster's state after
     a test.

     __Note__, the file has contain __only digits__, and nothing
     else. Even a trailing newline after the digits prevents the
     system from recognizing the request.
