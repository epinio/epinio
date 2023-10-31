---
status: {proposed}
date: {2023-10-13}
---

# Authorization Roles

## Authorization

Until version `v1.10.0` Epinio has a very simple authorization mechanism. A user can be `admin` or `user`.  
Admins can access everything, while users are restricted to the namespaces they created or they were given permission to.

This was pretty simple to implement but not very flexible.

## Decision Drivers

A more flexible authorization is requested (https://github.com/epinio/epinio/issues/1946).

Some of the requirements:
- Be easy to understand and administer
- Provide least privileges
- Allow separate roles for listing, reading details, editing, and creating
- Allow soft multi-tenancy across namespaces (e.g. user A has read/write access to apps in one namespace, but read-only access on another namespace)
- Not require too many changes to the existing code base

## Considered Options

- Secret with whitelist ACL per namespace
- Role Bindings as their own CRDs
- ConfigMaps defined Roles with predefined Actions

## Decision Outcome

Chosen option 3 (__"ConfigMaps defined Roles with predefined Actions"__). It is believed to be the easiest and most flexible solution. It will not require many changes in the code, and it will be easy to customize and implement.
We could also define some basic roles that can be used.


## Pros and Cons of the Options

### Secret with whitelist ACL per namespace

A secret per namespace containing the whitelist ACL could provide this without too much administrative overhead.

The ACL data in the secret could look something like this

```yaml
application:
  jwt_claim_name: rwlcd
  jwt_claim2: l
service:
  jwt_claim_name: rw
  jwt_claim2: rld
```

Where the RHS of the claim mapping is a string with a combination of letters:

```
r -- read
w -- write/edit
l -- list
c -- create
d -- delete
a -- all
```

A user with multiple claims would get access to the union of access and an admin would have access to everything.

Additionally, as long as the secret/configmap is mounted in the pod without any subpath, changing it would not require a restart of the API.

#### Pros

* Clear "resource" and "permission" description

#### Cons

* Not sure how to handle newly created namespaces
* Some operations are not strictly tied to a "resource". How to avoid the `exec` but allowing the creation of apps?

### Role Bindings as their own CRDs

Building custom Role Bindings as their own CRDs.

#### Pros

* Clear and structured implementation

#### Cons

* Difficult/complex to implement and evolve
* Complex to manage

### ConfigMaps defined Roles with predefined Actions

To avoid changing the current implementation too much we can add "roles" to an Epinio user. While a user can only have one role at the moment, this can be extended in the future with a list instead of a single value. Furthermore a role can be namespace-scoped by adding a `:` delimiter.

The Secret with the definition of the User can contain an annotation with the assigned roles.

```yaml
metadata:
  annotations:
    epinio.io/roles: epinio-role-reader,admin:workspace
  labels:
    epinio.io/role: user
```

*Note:*
An annotation will be used because labels have length-limitations and we don't want to be restricted about the number of roles. Also we don't need to perform any kind of lookup over the role.

In the previous example the user has the roles `epinio-role-reader` and `admin:workspace`. When working in the  namespace `workspace` the `admin` role will be used, otherwise the role `epinio-role-reader`.

A `Role` is defined as a kubernetes `ConfigMap` (no need to be a `Secret`) with a special label (`epinio.io/role: "true"`). This resource contains the `actions` that the role can perform, and some metadata that can be used for descriptions (i.e.: the `name`):

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    epinio.io/role: "true"
  name: epinio-role-namespace
  namespace: epinio
data:
  id: namespace-role
  name: Epinio Namespace Role
  default: "true"
  actions: |
    namespace
    app_read
    configuration_read
```

#### Fields

| Key     | Description 
|---------|-------------
| id      | The ID of the Role
| name    | A friendly name for the Role
| default | (optional) if set to _true_ the role will be the one selected as default if no other roles were assigned to the user
| actions | The actions the roles can perform

#### Actions

The actions are **hardcoded** as an embedded yaml file. This simplifies their management, and also enhances flexibility. An action can have some "dependencies", i.e.: the `namespace` action is a union of the `namespace_read` and `namespace_write`.

Every action lists the set of endpoints it allows. Note that some Epinio operations are formed from multiple endpoints, i.e. the `app push` consists of a Create, Update and others.

The following actions are the one defined in the `actions.yaml` file (in the `internal/auth` package).

##### Namespace

These actions enable operations on Namespace commands and resources.

| Action ID         | Description 
|-------------------|-------------
| `namespace_read`    | Read permissions (list, show)
| `namespace_write`   | Write permissions (create, delete)<br/>Depends on: `namespace_read`
| `namespace`         | All the above<br/>Depends on: `namespace_read`, `namespace_write`

##### App

These actions enable operations on App commands and resources. They also enable commands related to  AppCharts (`epinio app chart`) and application environment variables.

| Action ID       | Description 
|-----------------|-------------
| `app_read`        | Read permissions (app list and show, env list and show)
| `app_logs`        | Read application logs
| `app_write`       | Write permissions (app create, delete, push, export, stage, env set and unset)<br/>Depends on: `app_read`, `app_logs`
| `app_exec`        | Perform an exec into a running application
| `app_portforward` | Open a tunnel with the `port-forward` command
| `app`             | All the above<br/>Depends on: `app_read`, `app_logs`, `app_write`, `app_exec`, `app_portforward`

##### Configuration

These actions enable operations on Configuration commands and resources. Be aware that to bind a configuration you still need the `app_write` permission as well.


| Action ID           | Description 
|----------------------|-------------
| `configuration_read`  | Read permissions (list, show)
| `configuration_write` | Write permissions (create, delete)<br/>Depends on: `configuration_read`
| `configuration`       | All the above<br/>Depends on: `configuration_read`, `configuration_write`

##### Service

These actions enable operations on Service commands and resources. 

| Action ID             | Description 
|-----------------------|-------------
| `service_read`        | Read permissions (list, show)
| `service_write`       | Write permissions (create, delete, bind, unbind)<br/>Depends on: `service_read`
| `service_portforward` | Open a tunnel with the `port-forward` command
| `service`             | All the above<br/>Depends on: `service_read`, `service_write`, `service_portforward`

##### Gitconfig

These actions enable operations on Gitconfig commands and resources.

| Action ID         | Description 
|-------------------|-------------
| `gitconfig_read`    | Read permissions (list, show)
| `gitconfig_write`   | Write permissions (create, delete)<br/>Depends on: `gitconfig_read`
| `gitconfig`         | All the above<br/>Depends on: `gitconfig_read`, `gitconfig_write`

##### Export Registries

This action enables operations on Export Registries commands and resources. Only read operations are available.

| Action ID                 | Description 
|---------------------------|-------------
| `export_registries_read`  | Read permissions


#### Pros

* Easy to understand and manage
* Fairly easy to implement

#### Cons

* Actions to be defined manually
