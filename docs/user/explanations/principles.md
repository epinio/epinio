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