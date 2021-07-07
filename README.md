# Epinio

Opinionated platform that runs on Kubernetes, that takes you from App to URL in one step.

![CI](https://github.com/epinio/epinio/workflows/CI/badge.svg)

<img src="./docs/epinio.png" width="50%" height="50%">

## Contents

- [Epinio](#epinio)
  - [Contents](#contents)
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

## Installation

- [Install on K3d](./docs/user/tutorials/install_epinio_on_k3d.md) - Install K3d and then install epinio.
- [Install on GKE](./docs/user/tutorials/install_epinio_on_gke.md) - Install Epinio in GKE.

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