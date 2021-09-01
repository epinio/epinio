# Epinio, CLI configuration

The Epinio command line client uses a configuration file to store
information which has to persist across invocations. This document
discusses various aspects around this.

## Table Of Contents

  - [Location](#location)
  - [Contents](#contents)
  - [Commands](#commands)

## Location

Epinio's configuration is located by default at `~/.config/epinio/config.yaml`.

The location can be changed from command to command by specifying a
different path with the global command line option `--config-file`.

A more permanent change is possible by setting the environment
variable `EPINIO_CONFIG` to the desired path.

## Contents

Epinio's configuration contains

  - The name of the namespace currently targeted.
  - Epinio API user name
  - Epinio API password
  - Epinio API urls (regular and websocket)
  - Epinio API certificate

The namespace can be changed by running `epinio target` with the
name of the desired namespace as its single argument.

User name and password are used by the client to authenticate itself
when talking to Epinio's API server. The `epinio install` command
saves the initial information to the configuration.

The installation uses a the wildcard domain `omg.howdoi.website` and the
`epinio-ca` issuer by default.

`epinio install` saves the associated CA
certificate to the configuration so that future invocations of the
client are able to verify the actual certificate when talking to
Epinio's API server.

## Commands

The Epinio command line client currently provides 3 commands
explicitly targeting the configuration. These are:

  1. `epinio target`

     As noted in the previous section, this command changes
     the namespace to use with all other commands.

  2. `epinio config show`

     This command shows the details of the currently stored
     configuration settings. An exception is made for the certificate
     field, due to its expected size. The command's output only notes
     if certificate data is present or not.

  3. `epinio config update`

     Epinio allows users to switch between multiple installations (on
     different clusters) by simply re-targeting the cluster to talk to
     via changes to the  environment variable `KUBECONFIG`.

     When such is done the credentials, cert data, and urls stored in
     the configuration will not match the newly targeted cluster,
     except by coincidence.

     To be actually able to talk to the newly targeted installation it
     is necessary to run this command to refresh the stored
     credentials, cert data, and urls with information retrieved from
     the new cluster.
