# How To Add a New User For API Access

## Background and specification

Epinio users are stored in Kubernetes as secrets of type "BasicAuth". A secret
of that type in the `epinio` namespace, can be used to authenticate with
the Epinio API, as long as it has the following label:

```
epinio.suse.org/api-user-credentials=true
```

The `epinio install` command creates a default user for you with auto-generated
credentials. You can find your current user's credentials with the command:

```
epinio config show
```

## Adding a new user

Given the previous information the process of adding a new user "FantasticUser"
with password "FantasticPassword" able to access the Epinio API server is as follows:

1. Create the User description as a yaml:

```
# fantasticuser.yaml
--
apiVersion: v1
stringData:
  username: FantasticUser
  password: FantasticPassword
kind: Secret
metadata:
  labels:
    epinio.suse.org/api-user-credentials: "true"
  name: fantastic-epinio-user
  namespace: epinio
type: BasicAuth
```

(the name of the secret doesn't matter, choose something that makes sense to you)

2. Create the Secret on the cluster:

```
kubectl apply -f fantasticuser.yaml
```

Now you can edit your `~/.config/epinio/config.yaml` and set `pass` and `user`
to the new credentials above. You can delete all users and and new ones at any
time.

## NOTE

The admin command `epinio config update` updates the epinio `config.yaml`
with the credentials of the "older" user, based on creation date.
