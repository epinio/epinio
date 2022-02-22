!!!! I AM AN EVIL CONTRIBUTOR !!!!

# Epinio

Opinionated platform that runs on Kubernetes to take you from Code to URL in one step.

[![godoc](https://pkg.go.dev/badge/epinio/epinio)](https://pkg.go.dev/github.com/epinio/epinio/internal/api/v1)
[![CI](https://github.com/epinio/epinio/workflows/CI/badge.svg?event=schedule)](https://github.com/epinio/epinio/actions/workflows/main.yml?query=event%3Aschedule)
[![AKS-CI](https://github.com/epinio/epinio/actions/workflows/aks.yml/badge.svg?event=schedule)](https://github.com/epinio/epinio/actions/workflows/aks.yml?query=event%3Aschedule)
[![AKS-LETSENCRYPT-CI](https://github.com/epinio/epinio/actions/workflows/aks-letsencrypt.yml/badge.svg?event=schedule)](https://github.com/epinio/epinio/actions/workflows/aks-letsencrypt.yml?query=event%3Aschedule)
[![EKS-CI](https://github.com/epinio/epinio/actions/workflows/eks.yml/badge.svg?event=schedule)](https://github.com/epinio/epinio/actions/workflows/eks.yml?query=event%3Aschedule)
[![GKE-CI](https://github.com/epinio/epinio/actions/workflows/gke.yml/badge.svg?event=schedule)](https://github.com/epinio/epinio/actions/workflows/gke.yml?query=event%3Aschedule)
[![GKE-LETSENCRYPT-CI](https://github.com/epinio/epinio/actions/workflows/gke-letsencrypt.yml/badge.svg?event=schedule)](https://github.com/epinio/epinio/actions/workflows/gke-letsencrypt.yml?query=event%3Aschedule)
[![RKE-CI](https://github.com/epinio/epinio/actions/workflows/rke.yml/badge.svg?event=schedule)](https://github.com/epinio/epinio/actions/workflows/rke.yml?query=event%3Aschedule)
[![golangci-lint](https://github.com/epinio/epinio/actions/workflows/golangci-lint.yml/badge.svg?event=schedule)](https://github.com/epinio/epinio/actions/workflows/golangci-lint.yml?query=event%3Aschedule)
[![UI-SCENARIO-1-CHROME](https://github.com/epinio/epinio-end-to-end-tests/actions/workflows/scenario_1_cypress_chrome.yml/badge.svg?event=schedule)](https://github.com/epinio/epinio-end-to-end-tests/actions/workflows/scenario_1_cypress_chrome.yml?query=event%3Aschedule)
[![UI-SCENARIO-2-FIREFOX](https://github.com/epinio/epinio-end-to-end-tests/actions/workflows/scenario_2_cypress_firefox.yml/badge.svg?event=schedule)](https://github.com/epinio/epinio-end-to-end-tests/actions/workflows/scenario_2_cypress_firefox.yml?query=event%3Aschedule)

<img src="./docs/epinio.png" align="right" width="200" height="50%">

## Contents

- [Epinio](#epinio)
  - [Contents](#contents)
  - [What problem does Epinio solve](#what-problem-does-epinio-solve)
  - [Documentation](#documentation)
  - [Features](#features)
  - [Usage](#usage)
  - [Buildpacks](#buildpacks)
  - [Reach Us](#reach-us)
  - [Contributing](#contributing)
  - [License](#license)

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

Installation and user documentation is available at our main [docs.epinio.io](https://docs.epinio.io/) site.

Our [developer documentation](./docs) explains how to build and run Epinio from a source checkout.

## Features

- **Security**
  - mTLS: Epinio uses `linkerd` to secure all communication between epinio components inside the kubernetes cluster
  - Basic Authentication to access the API
- **Epinio Clients**
  - Web UI
  - Epinio CLI
- **Apps**
  - Push code directly without additional tools or steps
  - Basic operation of your application once pushed
  - Cloud Native Buildpacks build and containerize your code for you
- **Services**
  - CRUD operations of your service. A service can be a database, SaaS etc. A service can be an external component or can be created using `epinio service`
  - Bind services to apps

## Usage

- [QuickStart](https://docs.epinio.io/tutorials/quickstart.html) - Tutorial on how to create a namespace and push an application.

## Buildpacks

Buildpacks convert your application source code into container images in which the buildpack provides the framework, dependencies and runtime support for your app based on it's programming language.

Epinio uses [Paketo Buildpacks](https://paketo.io/docs/) through kubernetes jobs to convert your source code into container images. 

Epinio uses the Paketo's full [Builder Image](https://paketo.io/docs/concepts/builders/) by default.

[Using Custom Buildpack](./docs/developer/howtos/custom-python-builder.md) - Steps to create and use a custom builder image that includes a buildpack for Python (The paketo full [Builder Image](https://paketo.io/docs/concepts/builders/) doesn't support python apps yet).

### Example apps

- Rails: https://github.com/epinio/example-rails
- Java: https://github.com/spring-projects/spring-petclinic/
- Paketo Buildpack example apps: https://github.com/paketo-buildpacks/samples

## Reach Us

- Slack: #epinio on [Rancher Users](https://rancher-users.slack.com/)
- Github: [Discuss](https://github.com/epinio/epinio/discussions/new)

## Contributing

`Epinio` uses [Github Project](https://github.com/epinio/epinio/projects/1) for tracking issues. You can also find the issues currently being worked on in the `BackLog` section.

Find more information in the [Contribution Guide](./CONTRIBUTING.md).

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
