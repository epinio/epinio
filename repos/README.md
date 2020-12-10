# Repo CRD

A custom resource definition for Kubernetes.

- it allows the user to define a resource where they can store application sources and/or artifacts
- once the resource is created, the user is presented with a protocol and location for uploading (in status)
- the status of the resource reflects current revision and readiness (whether a transaction is still running or not)
- the resource has a list of revisions
