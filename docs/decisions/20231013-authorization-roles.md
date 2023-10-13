---
status: {proposed}
date: {2023-10-13}
---

# Authorization Roles

## Authorization

Until version `v1.10.0` Epinio has a very simple authorization mechanism. A user can be `admin` or `user`.  
An admin can perform everything, while a user can do anything but only in the namespaces he created or where has permissions.

This was pretty simple to implement but not very flexible.

## Decision Drivers

A new more flexible authorization was needed (https://github.com/epinio/epinio/issues/1946).

Some of the requirements:
- Be easy to understand and administer
- Provide least privileges
- Allow separate roles for listing, reading details, editing, and creating
- Allow soft multi-tenancy across namespaces (e.g. user A has read write access for apps on one namespace but read only on another namespace)
- Not require too many changes to the existing code base

## Considered Options

- Secret with whitelist ACL per namespace
- Role Bindings as their own CRDs
- ConfigMaps defined Roles with predefined Actions

## Decision Outcome

Chosen option 3 (__"ConfigMaps defined Roles with predefined Actions"__). It was the easier and more flexible solution. It will not require many changes in the code, and it will be easy to customize and implement.
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

* Not sure how to handle newly created namespace
* Some operations are not strictly tight to a "resource". How to avoid the `exec` but allowing the creation of apps?

### Role Bindings as their own CRDs

Building custom Role Bindings as their own CRDs.

#### Pros

* Clear and structured implementation

#### Cons

* Difficult/complex to implement and evolve
* Complex to manage

### ConfigMaps defined Roles with predefined Actions

To avoid changing too much the current implementation we can add "roles" to an Epinio user. At the moment the user can only have one role, but this can be extended with a list instead of a fixed value. Also the role can be namescoped adding a `::` delimiter.

The Secret with the definition of the User can contain an annotation with the assigned roles.

```yaml
metadata:
  annotations:
    epinio.io/roles: epinio-role-reader,admin::workspace
  labels:
    epinio.io/role: user
```

*Note:*
an annotation will be used because labels have some limitations about the lenght and we don't want to be restricted about the number of roles. Also we don't need to perform any kind of lookup over the role.

In the previous example the user will have the `epinio-role-reader` and `admin::workspace` roles. When working in the workspace namespace the `admin` role will be used, otherwise the `epinio-role-reader` will take over.

A Role can be simply defined as a ConfigMap (no need to be a secret) with a special label (`epinio.io/role: "true"`). This role contains the `actions` that this role can perform, and some metadata that can be used as descriptions (i.e.: the `name`):

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    epinio.io/role: "true"
  name: epinio-role-namespace
  namespace: epinio
data:
  name: Epinio Namespace Role
  actions: |
    namespace
```

This actions could be **hardcoded** in a yaml file. This simplify the management of them, and also the flexibility. An action can have some "dependencies", i.e.: the `namespace` action is a union of the `namespace_show`, `namespace_delete`, and so on.

Every action will have a set of endpoints that will allow. Some Epinio operations are formed by multiple endpoints, i.e. the `app_push` consists of a Create, Update and others.

Example of part of the `actions.yaml` file:

```yaml
# Namespace related actions
- id: namespace
  name: Namespace
  dependsOn:
    - namespace_list
    - namespace_show
    - namespace_create
    - namespace_delete
# Namespace List
- id: namespace_list
  name: Namespace List
  routes:
    - Namespaces
# Namespace Show
- id: namespace_show
  name: Namespace Show
  routes:
    - NamespaceShow
    - NamespacesMatch
    - NamespacesMatch0
```

This mapping can be used to match closer to the actions/commands performed from the CLI.

#### Pros

* Easy to understand and manage
* Fairly easy to implement

#### Cons

* Actions to be defined manually
