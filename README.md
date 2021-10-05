# Epinio

Opinionated platform that runs on Kubernetes, that takes you from App to URL in one step.

[![godoc](https://pkg.go.dev/badge/epinio/epinio)](https://pkg.go.dev/github.com/epinio/epinio/internal/api/v1)
[![CI](https://github.com/epinio/epinio/workflows/CI/badge.svg?event=schedule)](https://github.com/epinio/epinio/actions/workflows/main.yml?query=event%3Aschedule)
[![AKS-CI](https://github.com/epinio/epinio/actions/workflows/aks.yml/badge.svg?event=schedule)](https://github.com/epinio/epinio/actions/workflows/aks.yml)
[![EKS-CI](https://github.com/epinio/epinio/actions/workflows/eks.yml/badge.svg?event=schedule)](https://github.com/epinio/epinio/actions/workflows/eks.yml)
[![golangci-lint](https://github.com/epinio/epinio/actions/workflows/golangci-lint.yml/badge.svg?event=schedule)](https://github.com/epinio/epinio/actions/workflows/golangci-lint.yml)

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

## Documentation

Find installation and user documentation at [docs.epinio.io](https://docs.epinio.io/).

Find Epinio developer documentation here in [docs](./docs).

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

## Usage

- [QuickStart](https://docs.epinio.io/tutorials/quickstart.html) - tutorial on how to create an org and push an application.

## Buildpacks

Buildpacks convert your application source code into container images in which the buildpack provides the framework, dependencies and runtime support for your app based on it's programming language.

Epinio uses [Paketo Buildpacks](https://paketo.io/docs/) through tekton pipelines to convert your source code into container images. 

[Tekton Buildpack Pipeline](https://github.com/tektoncd/catalog/blob/main/task/buildpacks/0.3/buildpacks.yaml) - Epinio uses this tekton pipeline with the Paketo's full [Builder Image](https://paketo.io/docs/concepts/builders/).

[Using Custom Buildpack](./docs/developer/howtos/custom-python-builder.md) - Steps to create and use a custom builder image that includes a buildpack for Python (The paketo  full [Builder Image](https://paketo.io/docs/concepts/builders/) doesn't support python apps yet).

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
