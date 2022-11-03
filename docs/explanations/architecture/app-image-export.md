# Application image export

The `epinio app export` command can be used to export all the needed part of an application (the helm chart, its values and the container image).

The [`skopeo`](https://github.com/containers/skopeo) cli is used to download the image. This was choose over the standard `docker` cli because it doesn't need a deamon to run, it doesn't require root for most of its operations and it's OCI compliants.

When a client requests the export of the image of an application to the Epinio server, it will execute a Kubernetes job. This job will run the `skopeo copy` command that will fetch the image from the registry. The image will be downloaded on the PVC shared with the Epinio server. Once downloaded the image will beserved to the client that requested it.

A cleanup will occur if all the operations will succeed. The job will be not removed if some error occurs, to keep the logs for further investigations.

![app image export](app-image-export.png)