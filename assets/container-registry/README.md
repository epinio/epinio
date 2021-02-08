# Container Registry

This is the in-cluster Container Registry for hosting application images. It is
a simple Docker Registry deployment with Nginx as a reverse proxy for dealing
with TLS.

The TLS certificate is signed by the cluster CA, which is enough on most k8s
distributions to be able to pull images from this registry via a NodePort.
