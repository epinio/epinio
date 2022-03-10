## Design Principles

Epinio's development is governed by the following principles:

- **Greater Developer Experience**
  - Single command push for short learning curve
- **Edge Friendly** 
  - Has to fit in less than 4GB of RAM
- **Easy Installation** 
  - Has to install in less than 5 minutes when images are warm
  - Has to completely uninstall and leave the cluster in its previous state with a one-line command
- **Minimum Complexity**
  - Scale from desktop/local to data center environment
  - Has to install with a one-line command and zero config
- **API Driven Architecture**
- **Security Focused**

### Guidelines (Soft Principles)

- When possible, prefer:
  - components that are written in go
  - Kubernetes primitives over custom resources
  - Well known components with active community over custom code
- all acceptance tests should run in less than 10 minutes
- all tests should be able to run on the minimal cluster

## Features 

- **Security**
  - mTLS: Epinio uses `linkerd` to secure all communication between epinio components inside the kubernetes cluster
  - Basic Authentication to access the API.
- **Epinio Clients**
  - Web UI
  - Epinio CLI
- **Full Air-Gap Installation**
  - Can be installed and be used without internet
- **Apps**
  - CRUD operations of your app. (An app can be a tarball or in a github repo)
  - Cloud Native Buildpacks provide the runtime environment for your apps
- **Configurations**
  - CRUD operations of your configuration. A configuration can be a database, SaaS etc. A configuration can be an external component or can be created using `epinio configuration`
  - Bind configurations to apps.

## Future Plans

Epinio's development is driven by real world problems. That means, if something
is not a solution to a real user's problem then it is not a priority. This guideline
helps avoid over-engineering and meaningless work.

Epinio's main goal is to make existing solutions accessible, to the application
developer. Those solutions include:

- TLS Certificate Signers
- Configuration mesh integration and testing
- Open Telemetry
- Multi-Tenancy
- Serverless/Eventing framework
- Supply Chain Secruity
- Authorization/Authentication

With so many communities working on the same problems at the same time, it's rare
that a problem doesn't already have a solution. Most of the time, seamless integration
is the challenge and that's Epinio's domain.

__Note however__ that the above list does __not__ constitute a Roadmap of things
we will do. It is only a list of areas of interest to investigate.

You can see what the team is up to on our
[Github Project board](https://github.com/epinio/epinio/projects/1).
Feel free to add issues if you want to discuss future work or comment on
existing ones.