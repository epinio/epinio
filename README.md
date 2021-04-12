# epinio

Opinionated platform that runs on Kubernetes, that takes you from App to URL in one step.

![CI](https://github.com/epinio/epinio/workflows/CI/badge.svg)

<img src="./docs/epinio.svg" width="50%" height="50%">

## Principles

- must fit in less than 4GB of RAM
- must install in less than 5 minutes when images are warm
- must install with a one-line command and zero config
- must completely uninstall and leave the cluster in its previous state with a one-line command
- must work on local clusters (edge friendly)

### Guidelines

- if possible, choose components that are written in go
- all acceptance tests should run in less than 10 minutes
- all tests should be able to run on the minimal cluster 

## Usage
### Install

```bash
$ epinio install
```
### Uninstall

```bash
$ epinio uninstall
```

### Push an application

Run the following command for any supported application directory (e.g. inside [sample-app directory](sample-app)).

```bash
$ epinio push NAME PATH_TO_APPLICATION_SOURCES
```

Note that the path argument is __optional__.
If not specified the __current working directory__ will be used.
Always ensure that the chosen directory contains a supported application.

### Delete an application

```bash
$ epinio delete NAME
```

### Create a separate org

```bash
$ epinio create-org NAME
```

### Target an org

```bash
$ epinio target NAME
```

### List all commands

```bash
$ epinio help
```
### Detailed help for each command

```bash
$ epinio COMMAND --help
```

## Configuration

Epinio places its configuration at `$HOME/.config/epinio/config.yaml` by default.

For exceptional situations this can be overriden by either specifying

  - The global command-line option `--config-file`, or

  - The environment variable `EPINIO_CONFIG`.
