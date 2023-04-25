---
status: proposed
date: 2023-04-24
deciders: Andreas Kupries, Enrico Candino, Richard Cox
consulted: Richard Cox, Enrico Candino, Olivier Vernin
informed: Sorin Curescu, Francesco Torchia
---

# Extended Application Manifest and Server API in support of Web UI features for application management

## Context and Problem Statement

The Web UI manages extended application origin information and currently persists this information
in a special environment variable, to the likely confusion of users. This environment variable is
also brittle when accessing the application from both Web UI and CLI, causing this data to be easily
lost, to the detriment of users.

Further both Web UI and CLI are out of sync with respect to the application manifest, i.e. the
structure describing an application to users. In that Web UI and CLI use/export different formats
(JSON vs YAML).

See [Epinio PR 2221](https://github.com/epinio/epinio/pull/2221) for the main discussion.

## Decision Drivers

  * Converging Web UI and CLI to the same format for application manifests.
  * Stable persistence for the additional application information.

## Considered Options

  * For the converged application manifest format only one option is considered:

      1. Creation of a new API endpoint delivered the manifest in the desired final format. This
      	 moves the responsibility for the generation of the format from the clients to the Epinio
      	 server, and trivially to the desired outcome of having all clients export the same format.

  * For the persistence of the new application information two options were considered:

      1. Saving to a new adjunct kubernetes secret resource.
      2. Extending the application CRD with fields for the new application information.

## Decision Outcome

We are extending the application CRD with fields for the new application information.

While both persistence options will lead to stable persistence the chosen option is believed to be
likely less complex, as only the code reading and writing application CR's has to be adapted to
(extended for) the new information. Whereas with a separate resource completely new code will be
required, not just for reading and writing, but also construction and destruction.

## Validation

The implementation of both API endpoint and extended application information will be validated
through additional API and command tests added to the testsuites used by the CI/CD system.

## Specification Details

### API endpoint

The new API endpoint is an extension of the existing `AppPart` endpoint, i.e. of
`/namespaces/:namespace/applications/:app/part/:part`.

This endpoint is extended with a new allowed part value `manifest`, which, when requested delivers a
`application/octet-stream` result containing the application manifest serialized into its final form.

The chosen form is the YAML-formatted manifest currently written by the Epinio CLI.

This matches the behaviour of the existing part `values`, both in content format, and content type.

### Extended Application CRD

The extended application information managed by the Web UI revolves around the application origin,
where the Web UI provides more features to the user than the CLI.

The existing `origin` fields in the CRD are for the handling of paths, containers, and git
references.

In the case of paths the Web UI enables the user to make use of a variety of archive files, whereas
the CLI only supports directories. To this end a new field `archive` is added to the `origin`
object, of type `boolean`. True indicates that the path refers to some kind of (compressed) archive,
instead of a directory.

No additional information is managed for containers.

For git references two additional fields are added to the `git` objects in the `origin`
object. These fields are `provider` and `branch`, both of type `string`.

The first field, `provider` provides (sic!) information about the provider of the git repository,
with expected values of `git`, `github`, and `gitlab`. Note however that this list of examples is
not considered to be exhaustive and is definitely not a list of the only allowed values for the
field.

The second field, `branch`, records the name of the repository branch the `revision` is part of, in
support of the Web UI's ability to show the user a selection of commits to choose from when creating
an application or editing it's source. This also helps telling the user where the source came from
(`main`, `v2.7.6`, etc.)

The names of the new fields in the CRD, i.e. `archive`, `provider`, and `branch` are translated as
they are to the YAML serialization for these fields. In contrast to the existing `repository` field,
which is translated to `url` in the YAML.

The modified part of the CRD is

```
origin:
  properties:
    archive:
      type: boolean
    container:
      type: string
    git:
      properties:
        branch:
          type: string
        provider:
          type: string
        repository:
          type: string
        revision:
          type: string
      required:
      - repository
      type: object
    path:
      type: string
  type: object
```
