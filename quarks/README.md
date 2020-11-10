This should only install `QuarksSecret` and `QuarksJob`.

```
helm repo add quarks https://cloudfoundry-incubator.github.io/quarks-helm/
helm install quarks/cf-operator --version 6.1.17+0.gec409fd7
```
