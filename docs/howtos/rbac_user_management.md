# RBAC User Management

This guide explains how operators can create and manage Epinio users with role-based access control (RBAC).

## Overview

Epinio supports predefined roles that control what users can do:

| Role ID | Description |
|---------|-------------|
| `view_only` | Read-only access to apps, configurations, services, gitconfigs, export registries |
| `application_developer` | Create and update applications; no delete; no configuration/service write |
| `application_manager` | Full application CRUD (create, update, delete) and runtime operations |
| `system_manager` | Application CRUD + read-only on configurations and services; no delete on configs/services |

## Enable RBAC Roles (Install Time)

By default, Epinio installs these role ConfigMaps. To disable them:

```bash
helm install epinio ... --set api.rbac.enabled=false
```

With `api.rbac.enabled=true` (default), end users can be assigned `application_manager`, `application_developer`, etc. With it disabled, only the default `user` and `blank` roles are available.

## Creating Users

Users are defined as Kubernetes Secrets in the Epinio namespace. Each user needs:

1. **Label**: `epinio.io/api-user-credentials: "true"`
2. **Annotation**: `epinio.io/roles: "<role1>,<role2>:<namespace>,..."` — the roles to assign
3. **Data**: `username` (plain text), `password` (bcrypt hash), optionally `namespaces` (newline-separated list)

### Example: Application Manager

```yaml
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: my-app-manager
  namespace: epinio
  labels:
    epinio.io/api-user-credentials: "true"
  annotations:
    epinio.io/roles: "application_manager,application_manager:my-namespace"
stringData:
  username: "app-manager"
  password: "<bcrypt-hash-of-password>"
  namespaces: |
    my-namespace
    other-namespace
```

### Role Annotation Format

- `application_manager` — global role (applies when no namespace-scoped role matches)
- `application_manager:my-namespace` — role scoped to `my-namespace`
- Multiple roles: comma-separated, e.g. `application_manager,view_only:audit-ns`

### Namespaces

For namespace-scoped actions (create app, list apps in namespace), the user's Secret must list allowed namespaces in the `namespaces` field (newline-separated). Users can only perform actions in namespaces listed there.

## Generating Passwords

Use bcrypt to hash passwords. Example with Python:

```python
import bcrypt
password = b"my-secure-password"
hashed = bcrypt.hashpw(password, bcrypt.gensalt()).decode("utf-8")
print(hashed)
```

## Verifying Roles

After creating a user, they can check their roles:

```bash
curl -k -u username:password "https://<epinio-url>/api/v1/me"
```

The response `roles` array should include the assigned roles (e.g. `application_manager`), not just the default `user` role.

## Troubleshooting

If a user gets "User unauthorized" when creating or deleting apps:

1. **Check the Secret annotation**: `kubectl get secret <secret-name> -n epinio -o jsonpath='{.metadata.annotations.epinio\.io/roles}'` — must include `application_manager` (or appropriate role), not `user`.
2. **Check namespaces**: The `namespaces` field must include the namespace where the user is trying to create apps.
3. **Check role ConfigMaps exist**: `kubectl get configmap -n epinio -l epinio.io/role=true` — should list `epinio-role-application-manager`, etc.
4. **Restart Epinio server** if ConfigMaps were added after install: `kubectl rollout restart deployment/epinio-server -n epinio`

For detailed troubleshooting steps, see the Epinio documentation or the RBAC troubleshooting guide in your Epinio source repository.
