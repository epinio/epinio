# Custom Python Builder

Python buildpacks are not yet part of the builder (paketobuildpacks/builder:full) used in Epinio.
If the application contains a `Procfile`, the procfile buildpack will detect that and try to run the commands from the `Procfile`, but no Python setup is done.

Supported sample apps can be found here:
* https://github.com/paketo-buildpacks/samples
* https://github.com/buildpacks/samples

Python sample apps can be found in the Paketo community Python repo:
* https://github.com/paketo-community/python/tree/main/integration/testdata

## Pack

By using `pack` and a `project.toml` it's possible to add the Python buildpack to a project:

```toml
[project]
id = "sample"
version = "0.1"

[build]

[[build.buildpacks]]
id = "paketo-community/python"
version = "0.4.2"
```

And then build the image, for any Python sample, by running `pack build test/pip -B paketobuildpacks/builder:full`.


## Tekton

However, that is not possible with the Tekton pipeline (https://github.com/tektoncd/catalog/tree/main/task/buildpacks/0.3) included in Epinio. It does not recognize the `project.toml`.

* https://github.com/buildpacks/lifecycle/issues/555
* https://github.com/haliliceylan/rfcs/blob/2152fc5c817d971b6ead2069d82c459f432a7acc/text/0000-prepare-phase.md

## Solution: Using a Custom Builder

We can create a custom builder that supports Python and point the Tekton pipeline to the custom builder.

```
git clone git@github.com:paketo-buildpacks/full-builder.git

patch -p1 <<EOF
diff --git a/builder.toml b/builder.toml
index f3a35fd..b228671 100644
--- a/builder.toml
+++ b/builder.toml
@@ -32,6 +32,10 @@ description = "Ubuntu bionic base image with buildpacks for Java, .NET Core, Nod
   uri = "docker://gcr.io/paketo-buildpacks/php:0.5.0"
   version = "0.5.0"

+[[buildpacks]]
+  uri = "docker://gcr.io/paketo-community/python:0.4.2"
+  version = "0.4.2"
+
 [[buildpacks]]
   uri = "docker://gcr.io/paketo-buildpacks/procfile:4.2.2"
   version = "4.2.2"
@@ -97,6 +101,12 @@ description = "Ubuntu bionic base image with buildpacks for Java, .NET Core, Nod
     id = "paketo-buildpacks/java"
     version = "5.9.1"

+[[order]]
+
+  [[order.group]]
+    id = "paketo-community/python"
+    version = "0.4.2"
+
 [[order]]

   [[order.group]]
EOF

pack builder create epicustombuilder:local --config builder.toml
```

Make the image `epicustombuilder:local` available to your cluster, e.g. with k3d:

```
k3d image import epicustombuilder:local
```

Edit the tekton pipeline and replace `paketobuildpacks/builder:full` with our custom builder `epicustombuilder:local`:

```
kubectl edit -n tekton-staging pipeline/staging-pipeline
```

Finally, use `epinio push` with one of the sample Python apps to see that it works.

## Reference

* Project descriptor: https://github.com/buildpacks/spec/blob/main/extensions/project-descriptor.md#projectlicenses
* RFC replace buildpack.yml: https://github.com/paketo-buildpacks/rfcs/blob/main/text/0003-replace-buildpack-yml.md
* RFC environment variable configuration of buildpacks: https://github.com/paketo-buildpacks/rfcs/blob/main/text/0026-environment-variable-configuration-of-buildpacks.md
* Setup a Tekton buildpack pipeline: https://buildpacks.io/docs/tools/tekton/
