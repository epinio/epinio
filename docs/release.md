# How to Release Epinio

First release epinio/epinio
* [ ] check CI is green in epinio/epinio
* [ ] push version tag (vX.X.X)
* [ ] wait for release, then edit/check release notes on Github

Now relesease both epinio/helm-charts
* [ ] look for updatecli PR, created by epinio/epinio release
* [ ] check that `version:` keys are incremented, manually change for major/minor releases
* [ ] check that `epinioChart:` in chart/epinio-installer/values.yaml is correct
* [ ] wait for chart CI to finish
* [ ] manually trigger chart release

# How to use a new Installer

If the installer binary changed:

* [ ] push tag in epinio/installer repo
* [ ] wait for updatecli PR in epinio/helm-charts
