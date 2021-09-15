#  AWS EKS configuration

## Create an EKS cluster

If you don't have an existing cluster, follow the [quickstart](https://docs.aws.amazon.com/eks/latest/userguide/getting-started.html) to create an EKS cluster.

## EKS Prerequisites

* We follow the documented recommendation of having at least 2 nodes for EKS
* Epinio v0.1.0 has been tested with AWS EKS incl. kubernetes version v1.20.7 and v1.21.2
* To do more extensive testing we recommend a 2 node cluster with "t3.xlarge" instances
* To just try out Epinio, e.g. 2 "t3a.large" are sufficient

#### Install

Installing Epinio in an EKS cluster is done in three steps.

Follow [Installation using a Custom Domain](./docs/user/tutorials/install_epinio_customDNS.md) for test and production environments.
