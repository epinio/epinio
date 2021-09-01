# Epinio

Opinionated platform that runs on Kubernetes, that takes you from App to URL in one step.

[![godoc](https://pkg.go.dev/badge/epinio/epinio)](https://pkg.go.dev/github.com/epinio/epinio/internal/api/v1)
[![CI](https://github.com/epinio/epinio/workflows/CI/badge.svg)](https://github.com/epinio/epinio/actions/workflows/main.yml?query=event%3Aschedule)
[![AKS-CI](https://github.com/epinio/epinio/actions/workflows/aks.yml/badge.svg?event=schedule)](https://github.com/epinio/epinio/actions/workflows/aks.yml)
[![EKS-CI](https://github.com/epinio/epinio/actions/workflows/eks.yml/badge.svg?event=schedule)](https://github.com/epinio/epinio/actions/workflows/eks.yml)
[![golangci-lint](https://github.com/epinio/epinio/actions/workflows/golangci-lint.yml/badge.svg?event=schedule)](https://github.com/epinio/epinio/actions/workflows/golangci-lint.yml)

<img src="./docs/epinio.png" align="right" width="200" height="50%">

## Contents

- [Epinio](#epinio)
  - [Contents](#contents)
  - [Features](#features)
  - [What problem does Epinio solve](#what-problem-does-epinio-solve)
  - [Installation](#installation)
  - [Usage](#usage)
  - [How the documentation is organized](#how-the-documentation-is-organized)
  - [Reach Us](#reach-us)
  - [Contributing](#contributing)
  - [License](#license)

## What problem does Epinio solve

Epinio makes it easy for developers to deploy their applications to Kubernetes. Easy means:

- No previous experience with Kubernetes is required
- No steep learning curve
- Quick local setup with zero configuration
- Deploying to production similar to development

Kubernetes is becoming the de-facto standard for container orchestration.
Developers may want to use Kubernetes for all the benefits it provides or may
have to do so because that's what their Ops team has chosen. Whatever the case,
using Kubernetes is not simple. It has a steep learning curve and doing it right
is a full time job. Developers should spend their time working on their applications,
not doing operations.

Epinio is adding the needed abstractions and intelligence to allow Developers
to use Kubernetes as a PaaS (Platform as a Service).

## Features

- **Security**
  - mTLS: Epinio uses `linkerd` to secure all communication between epinio components inside the kubernetes cluster
  - Basic Authentication to access the API.
- **Epinio Clients**
  - Web UI
  - Epinio CLI
- **Apps**
  - CRUD operations of your app. (An app can be a tarball or in a github repo)
  - Cloud Native Buildpacks provide the runtime environment for your apps
- **Services**
  - CRUD operations of your service. A service can be a database, SaaS etc. A service can be an external component or can be created using `epinio service`
  - Bind services to apps.

## Installation

### System Requirements

#### Kubernetes Cluster Requirements

For the Epinio server, and related deployments we recommend to consider the following resources:

- 2-4 VCPUs
- 8GB RAM (system memory + 4GB)
- 10GB Disk space (system disk + 5GB)

In addition, extensive requirements for your workload (apps) would add to that.

#### Epinio CLI

The Epinio CLI will typically run on a host, which will need network access to your kubernetes cluster.
Usually you will use the same host to run tooling, like e.g. "kubectl" and "helm".

The compiled binary will use about 40-50MB disk space, incl. local configuration files.

### Install the Epinio CLI

Refer to [Install the Epinio CLI](./docs/user/tutorials/install_epinio_cli.md).

### Installation Methods (in Cluster)

Beside advanced installation options, there are two ways of installing Epinio:

1. [Installation using a MagicDNS Service](./docs/user/tutorials/install_epinio_magicDNS.md)

- For test environments. This should work on nearly any kubernetes distribution. Epinio will try to automatically create a magic DNS domain, e.g. **10.0.0.1.omg.howdio.website**.

2. [Installation using a Custom Domain](./docs/user/tutorials/install_epinio_customDNS.md)

- For test and production environments. You will define a system domain, e.g. **test.example.com**.

### Installation on Specific Kubernetes Offerings

- [Install on K3d](./docs/user/tutorials/install_epinio_on_k3d.md) - Install K3d and then install epinio.
- [Install on GKE](./docs/user/tutorials/install_epinio_on_gke.md) - Install Epinio in GKE.
- [Install on EKS](./docs/user/tutorials/install_epinio_on_eks.md) - Install Epinio in Amazon EKS clusters.
- [Install on AKS](./docs/user/tutorials/install_epinio_on_aks.md) - Install Epinio in Azure AKS clusters.

## Usage

- [QuickStart](./docs/user/tutorials/quickstart.md) - tutorial on how to create an org and push an application.

## How the documentation is organized

Epinio documentation is organised into these four quadrants in the `./docs/` folder.

[Tutorials](./docs/user/tutorials/) take you by the hand through a series of steps that are useful for a beginner like how to install epinio in various kubernetes distros, how to push an application and an org.

[How-to-guides](./docs/user/howtos/) explain steps to solve specific problems like how to create a redis database using epinio etc.

[Explanations](./docs/user/explanations/) discuss components of Epinio at a very high level like about linkerd, traefik etc.

[References](./docs/user/references/) provides references about Epinio CLI docs and Epinio API docs.

## Reach Us

- Slack: #epinio on [Rancher Users](https://rancher-users.slack.com/)
- Github: [Discuss](https://github.com/epinio/epinio/discussions/new)

## Contributing

`Epinio` uses [Github Project](https://github.com/epinio/epinio/projects/1) for tracking issues. You can also find the issues currenlty being worked on in the `BackLog` section.

If you would like to start contributing to `Epinio`, then you can pick up any of the cards with label `good first issue`.

## License

Copyright (c) 2020-2021 [SUSE, LLC](http://suse.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
