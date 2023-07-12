# Epinio

Opinionated platform that runs on Kubernetes to take you from Code to URL in one step.

[![godoc](https://pkg.go.dev/badge/epinio/epinio)](https://pkg.go.dev/github.com/epinio/epinio/internal/api/v1)
[![Go Report Card](https://goreportcard.com/badge/github.com/epinio/epinio)](https://goreportcard.com/report/github.com/epinio/epinio)
[![CI](https://github.com/epinio/epinio/workflows/CI/badge.svg?branch=main)](https://github.com/epinio/epinio/actions/workflows/main.yml?query=branch%3Amain)
[![golangci-lint](https://github.com/epinio/epinio/actions/workflows/golangci-lint.yml/badge.svg?branch=main)](https://github.com/epinio/epinio/actions/workflows/golangci-lint.yml?query=branch%3Amain)
[![AKS-CI](https://github.com/epinio/epinio/actions/workflows/aks.yml/badge.svg?branch=main)](https://github.com/epinio/epinio/actions/workflows/aks.yml?query=branch%3Amain)
[![EKS-CI](https://github.com/epinio/epinio/actions/workflows/eks.yml/badge.svg?branch=main)](https://github.com/epinio/epinio/actions/workflows/eks.yml?query=branch%3Amain)
[![GKE-CI](https://github.com/epinio/epinio/actions/workflows/gke.yml/badge.svg?branch=main)](https://github.com/epinio/epinio/actions/workflows/gke.yml??query=branch%3Amain)
[![RKE-CI](https://github.com/epinio/epinio/actions/workflows/rke.yml/badge.svg?branch=main)](https://github.com/epinio/epinio/actions/workflows/rke.yml?query=branch%3Amain)  
[![RKE2-EC2-CI](https://github.com/epinio/epinio/actions/workflows/rke2-lh-ec2.yml/badge.svg?branch=main)](https://github.com/epinio/epinio/actions/workflows/rke2-lh-ec2.yml?query=branch%3Amain) 
[![AKS-LETSENCRYPT-CI](https://github.com/epinio/epinio/actions/workflows/aks-letsencrypt.yml/badge.svg?branch=main)](https://github.com/epinio/epinio/actions/workflows/aks-letsencrypt.yml?query=branch%3Amain)
[![GKE-LETSENCRYPT-CI](https://github.com/epinio/epinio/actions/workflows/gke-letsencrypt.yml/badge.svg?branch=main)](https://github.com/epinio/epinio/actions/workflows/gke-letsencrypt.yml?query=branch%3Amain)
[![GKE-UPGRADE-CI](https://github.com/epinio/epinio/actions/workflows/gke-upgrade.yml/badge.svg?branch=main)](https://github.com/epinio/epinio/actions/workflows/gke-upgrade.yml??query=branch%3Amain)
[![RKE-UPGRADE-CI](https://github.com/epinio/epinio/actions/workflows/rke-upgrade.yml/badge.svg?branch=main)](https://github.com/epinio/epinio/actions/workflows/rke-upgrade.yml?query=branch%3Amain)

[E2E tests](https://github.com/epinio/epinio-end-to-end-tests):

[![Rancher-UI-1-Chrome](https://github.com/epinio/epinio-end-to-end-tests/actions/workflows/scenario_1_chrome_rancher_ui.yml/badge.svg?branch=main)](https://github.com/epinio/epinio-end-to-end-tests/actions/workflows/scenario_1_chrome_rancher_ui.yml?query=branch%3Amain)
[![Rancher-UI-1-Firefox](https://github.com/epinio/epinio-end-to-end-tests/actions/workflows/scenario_2_firefox_rancher_ui.yml/badge.svg?branch=main)](https://github.com/epinio/epinio-end-to-end-tests/actions/workflows/scenario_2_firefox_rancher_ui.yml?query=branch%3Amain)
[![Standalone UI Chrome](https://github.com/epinio/epinio-end-to-end-tests/actions/workflows/std_ui_latest_chrome.yml/badge.svg?branch=main)](https://github.com/epinio/epinio-end-to-end-tests/actions/workflows/std_ui_latest_chrome.yml?query=branch%3Amain)
[![Standalone UI Firefox](https://github.com/epinio/epinio-end-to-end-tests/actions/workflows/std_ui_latest_firefox.yml/badge.svg?branch=main)](https://github.com/epinio/epinio-end-to-end-tests/actions/workflows/std_ui_latest_firefox.yml?query=branch%3Amain)

<img src="./docs/epinio.png" align="left" width="100" height="50%">

## What problem does Epinio solve?

Epinio makes it easy for developers to iterate on their applications running in Kubernetes. Easy means:

- No experience with Kubernetes is required
- No steep learning curve
- Quick local setup with minimal configuration
- Deploying to production similar to development

Kubernetes is becoming the de-facto standard for container orchestration.
Developers may want to use Kubernetes for all the benefits it provides or may
have to do so because that's what their Ops team has chosen. Whatever the case,
using Kubernetes is not simple. It has a steep learning curve and doing it right
is a full time job. Developers should spend their time working on their applications,
not doing operations.

Epinio is adding the needed abstractions and intelligence to allow Developers
to use Kubernetes as a PaaS (Platform as a Service).

## Documentation

Detailed installation and user documentation is available at our main
[docs.epinio.io](https://docs.epinio.io/) site.

### Installation

The basic boilerplate requires a Kubernetes cluster, an Ingress Controller and a Cert Manager as
detailed in the [documentation](https://docs.epinio.io/installation/install_epinio). Once this is in
place, and leaving out DNS setup, in the most trivial case the main installation boils down to

```
helm repo add epinio https://epinio.github.io/helm-charts
helm repo update

helm install --namespace epinio --create-namespace epinio epinio/epinio \                                               --set global.domain=mydomain.example.com
```

For the details glossed over here see the
[documentation](https://docs.epinio.io/installation/install_epinio).

### Client installation

Installation of the Epinio CLI can be as simple as downloading a binary from the
[release page](https://github.com/epinio/epinio/releases), or usage of `brew`, i.e.

```
brew install epinio
```

For the details glossed over here see the
[documentation](https://docs.epinio.io/installation/install_epinio_cli).

### Quick Start Tutorial

- Our [QuickStart Tutorial](https://docs.epinio.io/tutorials/quickstart) explains how to create a
  namespace and push an application.

### Reach Us

- Slack: #epinio on [Rancher Users](https://rancher-users.slack.com/)
- Github: [Discuss](https://github.com/epinio/epinio/discussions/new)

### Contributing

`Epinio` uses [Github Project](https://github.com/epinio/epinio/projects/1) for tracking issues.

Find more information in the [Contribution Guide](./CONTRIBUTING.md).

Our [developer documentation](./docs) explains how to build and run Epinio from a source checkout.

## Features

- **Security**
  - TLS secured API server
  - Basic Authentication to access the API
  - __or__ OIDC-based token
- **Epinio Clients**
  - Web UI
  - Epinio CLI
- **Apps**
  - Push code directly without additional tools or steps
  - Basic operation of your application once pushed
  - Cloud Native Buildpacks build and containerize your code for you
- **Configurations**
  - CRUD operations of your configuration. A configuration can be a database, SaaS etc. A configuration can be an external component or can be created using `epinio configuration`
  - Bind configurations to apps

## Example apps

- Rails: https://github.com/epinio/example-rails
- Java: https://github.com/spring-projects/spring-petclinic/
- Paketo Buildpack example apps: https://github.com/paketo-buildpacks/samples

## License

Copyright (c) 2020-2023 [SUSE, LLC](https://suse.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
