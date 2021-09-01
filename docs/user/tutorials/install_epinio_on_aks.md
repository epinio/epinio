# MS Azure AKS configuration

## Create an AKS cluster

If you don't have an existing cluster, follow the [quickstart](https://docs.microsoft.com/en-us/azure/aks/kubernetes-walkthrough) to create an AKS cluster.

## AKS Prerequisites

* Epinio v0.0.19 has been tested with Azure AKS incl. kubernetes version v1.20.7
* Epinio Acceptance Tests passed on a 2 node cluster with Standard_D3_v2 instances
* To just try out Epinio, e.g. 2 Standard_D2_v2 are sufficient

## Install

Beside advanced installation options, there are two ways of installing Epinio:

1. [Installation using a MagicDNS Service](./docs/user/tutorials/install_epinio_magicDNS.md)

- For test environments. This should work on nearly any kubernetes distribution. Epinio will try to automatically create a magic DNS domain, e.g. **10.0.0.1.omg.howdoi.website**.

2. [Installation using a Custom Domain](./docs/user/tutorials/install_epinio_customDNS.md)

- For test and production environments. You will define a system domain, e.g. **test.example.com**.
