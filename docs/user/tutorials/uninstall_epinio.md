### Uninstall

NOTE: The command below will delete all the components Epinio originally installed.
**This includes all the deployed applications.**

If after installing Epinio, you deployed other things on the same cluster
that depended on those Epinio deployed components (e.g. Traefik, Tekton etc),
then removing Epinio will remove those components and this may break your other
workloads that depended on these. Make sure you understand the implications of
uninstalling Epinio before you proceed.

If you want to completely uninstall Epinio from your kubernetes cluster, you
can do this with the command:

```bash
epinio uninstall
```