This is the main Epinio documentation page.

We split the docs into two folders: `developer/` and `user/`.
One contains information for developers of Epinio, like how to run tests or set up a local cluster.
The other folder has information for users of Epinio.

The structure of each doc directory is inspired by https://documentation.divio.com/structure/.

## User documentation

### [Tutorials](user/tutorials/)

Documentation that teaches practical skills for basic use-cases

- [Quickstart](user/tutorials/quickstart.md)
- [Install Epinio cli](user/tutorials/install_epinio_cli.md)
- [Install Epinio with custom DNS](user/tutorials/install_epinio_customDNS.md)
- [Install Epinio with "magic" DNS](user/tutorials/install_epinio_magicDNS.md)
- [Install Epinio with "magic" DNS](user/tutorials/install_epinio_magicDNS.md)
- [Install Epinio on AKS (Azure)](user/tutorials/install_epinio_on_aks.md)
- [Install Epinio on EKS (Amazon)](user/tutorials/install_epinio_on_eks.md)
- [Install Epinio on GKE (Google)](user/tutorials/install_epinio_on_gke.md)
- [Install Epinio on k3d (local)](user/tutorials/install_epinio_on_k3d.md)
- [Install Wordpress on Epinio](user/tutorials/install_wordpress_application.md)
- [Uninstall Epinio](user/tutorials/uninstall_epinio.md)

### [Explanations](user/explanations/)

Documentation that gives background information on key concepts

- [Advanced topics](user/explanations/advanced.md)
- [Configuration](user/explanations/configuration.md)
- [Detailed Push Process](user/explanations/detailed-push-process.md)
- [Principles](user/explanations/principles.md)
- [Security](user/explanations/security.md)
- [Windows](user/explanations/windows.md)

### [HowTos](user/howtos/)

Documentation about more practical and solve specific, sometimes advanced, problems

- [Certificate Issuers](user/howtos/certificate_issuers.md)
- [Provision external IP address for local Kubernetes](user/howtos/provision_external_ip_for_local_kubernetes.md)
- [Push with gitjob](user/howtos/gitjob_push.md)

### [References](user/references/)

Documenation containing information, like all CLI commands and their arguments

- [Command requirements](user/references/README.md)
- [Command reference](user/references/)
- [Supported Applications](user/references/supported-apps.md)

## Developer documentation

### [Explanations](developer/explanations/)

- [Future plans](developer/explanations/futureplans.md)

### [HowTos](developer/howtos/)

Practical Guides to solving practical problems.

- [Development Guidelines](developer/howtos/development.md)
- [Add a new API User](developer/howtos/new-api-user.md)
- [Run Acceptance test suite](developer/howtos/acceptance_tests.md)
- [Create and use a custom builder for Python](developer/howtos/custom-python-builder.md)
