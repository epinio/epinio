---
name: Release Checklist
about: Checklist for a new Epinio release
title: ''
labels: ''
assignees: ''

---

# Release vX.Y.Z

Checklist and steps to follow to publish an Epinio release.  

If you need more details you can find more information in the [Wiki](https://github.com/epinio/epinio/wiki/Releasing-Epinio).


## Epinio

* [ ] Check the CI status in `epinio/epinio`

  [![CI](https://github.com/epinio/epinio/workflows/CI/badge.svg?branch=main)](https://github.com/epinio/epinio/actions/workflows/main.yml?query=branch%3Amain)

* [ ] **( 📝 Manual step )** Edit [HERE](https://github.com/epinio/epinio/releases) the latest draft release, then publish the release.

* [ ] Check [HERE](https://github.com/epinio/epinio/actions/workflows/release.yml) the release action result.

* [ ] Check [HERE](https://github.com/epinio/epinio/releases) the release page for the latest assets and changelog.

* [ ] Check [HERE](https://github.com/epinio/homebrew-tap/blob/main/Formula/epinio.rb) that the `epinio/homebrew-tap` Formula was updated

* [ ] Check [HERE](https://github.com/Homebrew/homebrew-core/pulls?q=is%3Apr+epinio) that the `Homebrew/homebrew-core` has an open (or already closed) PR with the latest Epinio version.


## Helm Charts

* [ ] **( 📝 Manual step )** Check [HERE](https://github.com/epinio/helm-charts/pulls?q=is%3Apr+author%3Aapp%2Fgithub-actions) the `epinio/helm-charts` pull requests for the latest update, then merge the PR.

* [ ] **( 📝 Manual step )** Run [HERE](https://github.com/epinio/helm-charts/actions/workflows/release.yml) the `epinio/helm-charts` release action to publish the latest chart.


## Docs

* [ ] **( 📝 Manual step )** Check [HERE](https://github.com/epinio/docs/pulls?q=is:pr+author:app/github-actions) the `epinio/docs` pull requests for the latest update, then merge the PR.
