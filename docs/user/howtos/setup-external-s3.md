## How to setup external S3 storage

One of the steps involved in running `epinio push` is storing the requested version of the code
in the configured Epinio S3 compatible storage. By default, Epinio installs and configures [Minio](https://github.com/minio/minio)
to be used for this purpose. This document describes how to point Epinio to another S3 compatible storage and skip the Minio installation.

The [`epinio install` command](../references/cli/epinio_install.md), has the following optional parameters:


```
--s3-endpoint
--s3-access-key-id
--s3-secret-access-key
--s3-bucket
--s3-location
--s3-use-ssl
```

To configure Epinio to store application sources to an external S3 compatible storage, at least the following should be set:

```
--s3-endpoint
--s3-access-key-id
--s3-secret-access-key
--s3-bucket
```

the other 2 are optional:

```
--s3-location
--s3-use-ssl
```

(Some implementations don't need the location (e.g. Minio) and `s3-use-ssl` has a default value of "false")

An example that points Epinio to AWS S3 will look like this:

```
epinio install \
--s3-endpoint s3.amazonaws.com \
--s3-access-key-id your_access_key_here \
--s3-secret-access-key your_secret_key_here \
--s3-bucket epinio_sources \
--s3-location eu-central-1
--s3-use-ssl
```

If the bucket doesn't exist, Epinio will try to create it when it first tries
to write to it. Make sure the access key and the access secret have enough permissions
to create a bucket and then write to it.

When you successfully push a new version of your application, Epinio will remove the resources of the previous staging process from the Kubernetes cluster and
will also delete the previous version of the sources from S3. This way, Epinio doesn't store more than it needs on the S3 storage and the user doesn't need to manually cleanup.
