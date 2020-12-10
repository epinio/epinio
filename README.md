# carrier

![CI](https://github.com/SUSE/carrier/workflows/CI/badge.svg)

<img src="./docs/carrier.svg" width="50%" height="50%">

## Principles

- must fit in less than 4GB of RAM
- must install in less than 5 minutes
- must install with a one-line command 
- must completely uninstall and leave the cluster in its previous state with a one-line command
- must work on local clusters

### Guidelines

- if possible, choose components that are written in go
- all acceptance tests should run in less than 10 minutes

## Install

```bash
$ ./carrier install
```
