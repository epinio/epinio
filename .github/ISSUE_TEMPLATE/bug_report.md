---
name: Bug report
about: Create a report to help us improve
title: ''
labels:
  - kind/bug
assignees: ''

---

**Describe the bug**
A clear and concise description of what the bug is.

Hint: Make sure to search through the existing issues, as they often contain important information on recent changes.

**To Reproduce**
Steps to reproduce the behavior, e.g.:

1. Install Epinio to '...'
2. Push an app '....'
3. Look at cluster '....'
4. See error

**Expected behavior**
A clear and concise description of what you expected to happen.

**Logs**
If applicable, add logs to help explain your problem.

You can use attachments to add a screenshot of your clusters state, e.g. from k9s.

If you paste long logs, you could also add them into a collapsed block:

&lt;details&gt;
  &lt;summary&gt;Click to expand&lt;summary&gt;

  \```
  pasted log
  \```
&lt;details&gt;

You can increase Epinio's logging by exporting this variable:

```
export TRACE_LEVEL=255
```

To follow the server logs you can use:

`kubectl logs -n epinio -l app.kubernetes.io/name=epinio-server -c epinio-server -f`

**Cluster (please complete the following information):**
 - Provider: [e.g. K3D, minikube, KinD, AKS, EKS, GKE, RKE, ...]
 - Options: [e.g. number of nodes, storageclasses, loadbalancer if customised]
 - Kubernetes Version: [e.g. 1.20]

**Desktop/CLI (please complete the following information):**
 - OS: [e.g. Linux, MacOS, Windows]
 - Epinio Version: [e.g. 0.0.24]
 - Epinio Install Options: [e.g. loadbalancer, type of system domain]

**Additional context**
Add any other context about the problem here.
Anything special about your setup, like using a proxy or special DNS setup?
