# Tools definitions

This directory contains definitions for additional tools used to build the project.

There are 3 types of tools:

* internal tools are defined as aliases or (preferably) shell functions.

* callable tools are supposed to be already present on the filesystem.

* installable tools can be downloaded and installed if the existing version is not compatible.

## Common settings

A definition file can define one or more tools. At a minimum the tool names have to be added to the `TOOLS` array:

```
# shellcheck shell=bash
# shellcheck disable=SC2034

TOOLS+=(foo bar)
```

## Internal tools

Internal tools provide the tool definition inline. Optionally they can define additional prerequisites that should be checked/installed when this tool is required. For example:

```
TOOLS+=(xargs_no_run_if_empty)

function xargs_no_run_if_empty {
    if [ "${UNAME}" = "DARWIN" ]; then
        # macOS xargs doesn't support --no-run-if-empty; it is default behavior.
        xargs "$@"
    else
        xargs --no-run-if-empty "$@"
    fi
}

XARGS_NO_RUN_IF_EMPTY_REQUIRES="xargs"
```

`FOO_REQUIRES` is a string of space-separated names of additional tools required by the implementation of this tool. The name is the uppercase version of the tool name followed by `_REQUIRES`.

The `$UNAME` variable is set to `DARWIN`, `LINUX`, or `WINDOWS` to allow for platform specific specialization.

## Callable tools

Callable tools specify the minimum required version of the tool. They also may need to specify a function to return the version of the installed tool in case the default command (`$TOOL version`) does not provide the correct information.

```
TOOLS+=(ruby)

function ruby_version {
    ruby --version
}

RUBY_VERSION=2.4
```

The `foo_version` function does not need to isolate the semantic version; it just needs to return a string where the first match for a semantic version including at least one dot is the version of the tool. So `ruby 2.7.0p0` provided in the example is sufficient to determine the semantic version to be `2.7.0`.

## Installable tools

Installable tools need to provide a download URL in addition to the minimum required version. The URL can include the substring `{version}`, which will be replaced by the requested version and is just a convenience for maintaining the tool definition.

Optionally the definition may also include a SHA256 checksum for the downloaded file, that will be used to verify the integrity of the download.

```
TOOLS+=(jq)

JQ_SHA256_DARWIN="5c0a0a3ea600f302ee458b30317425dd9632d1ad8882259fcaf4e9b868b2b1ef"
JQ_SHA256_LINUX="af986793a515d500ab2d35f8d2aecd656e764504b789b66d7e1a0b727a124c44"
JQ_SHA256_WINDOWS="a51d36968dcbdeabb3142c6f5cf9b401a65dc3a095f3144bd0c118d5bb192753"

JQ_URL_DARWIN="https://github.com/stedolan/jq/releases/download/jq-{version}/jq-osx-amd64"
JQ_URL_LINUX="https://github.com/stedolan/jq/releases/download/jq-{version}/jq-linux64"
JQ_URL_WINDOWS="https://github.com/stedolan/jq/releases/download/jq-{version}/jq-win64.exe"

JQ_VERSION="1.6"

function jq_version { jq --version; }
```

In general the specified minimum version will only be downloaded when an already installed local version has a lower version number. In case the `PINNED_TOOLS` environment variable is set, an exact version match is required.
