# Manifest Support Internals

## Structures

A go structure `ApplicationManifest` contains all the manifest information. The YAML data
structures described in the user document map directly to structure fields and types.

The various commands (`create`, `update`, `push`) convert manifest data to the relevant
API structures, i.e. `ApplicationUpdateRequest` and `ApplicationCreateRequest`.

The fields of `ApplicationCreateRequest` are part of `ApplicationManifest`, by means of an
unnamed field. This means that changes to the former are automatically reflected in the
latter. Converting the latter to the former is trivial.

Similarly the fields of `ApplicationUpdateRequest` are part of `ApplicationCreateRequest`,
named however.

Manifest information not relevant to creation and/or updates is outside of the unnamed
parts, as proper named fields.

Regarding paths, all structures store only absolute paths in the respective fields. Any
relative paths are resolved as specified in the user documentation before being stored.

## Commands

The `create` and `update` commands do not process a manifest file, and only a subset of
the options, i.e. `--instances`, `--bind`, and `--env`.

Only the `push` command processes all options, and a manifest file.

This is implemented in three functions:

  - `GetManifest`

    Read/unmarshall a manifest file.

  - `UpdateManifestISE`

    Process `--instances`, `--bind`, and `--env` options (instances, services, ev's
    (ISE)) and merge the results into a manifest.

  - `UpdateManifestSN`

    Process `--name`, `--path`, `--git`, `--container-image` (sources, name (SN)) and
    merge the results into a manifest.

The `push` command uses all of the above to read an initial manifest from a file and then
updates it from the options.

The `create` and `update` commands on the other hand only use `UpdateManifestISE`.

  - `ManifestISEToUpdate`

    All commands need another function converting the parts of the manifest used by them
    all, i.e. instances, services, ev's (ISE) into an `ApplicationUpdateRequest`.

The `push` command uses the other information (sources, name (SN)) more directly itself.

## API

The `ApplicationUpdateRequest` API structure is modified with respect to environment
variables. Instead of a list of assignments a map is used, same as for the manifest
structures. This avoids the need of converting the client's map to a list and then
converting it back to a map in the server for storage.
