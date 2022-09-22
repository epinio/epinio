# Crossplane and Localstack

This guide will show you how to setup a local cluster (`k3d`) with [Crossplane](https://crossplane.io/) and [Localstack](https://localstack.cloud/), to be able to provision AWS resources locally with Epinio Services.

(TODO: link to docs/blog post)

## Kubernetes

We are going to use `k3d` to spin up our local cluster

```bash
$ k3d cluster create mycluster
```

## Localstack

Localstack is a cloud service emulator that runs on a docker container, so it can be used to mock AWS services on your laptop.  
Once installed we can start it in detach mode, and we can add the container in the k3d-mycluster network, so that we can reach Localstack from our cluster.

```bash
$ localstack start -d
$ docker network connect k3d-mycluster localstack_main
```

To check the networking we can find the localstack ip in the network with an inspect

```
$ docker inspect localstack_main | jq -r '.[].NetworkSettings.Networks["k3d-mycluster"].IPAddress'
172.18.0.4
```

and we can run a curl from a pod in our cluster

```bash
$ kubectl run --image=radial/busyboxplus:curl -it --rm curl
[ root@curl:/ ]$ curl -w "Status Code: %{http_code}\n" 172.18.0.4:4566
Status Code: 200
```


## Crossplane

To install Crossplane we can use Helm

```bash
$ helm repo add crossplane-stable https://charts.crossplane.io/stable
$ helm repo update
$ helm install crossplane --create-namespace --namespace crossplane-system crossplane-stable/crossplane
```

We can check the installation with `kubectl get all -n crossplane-system` or a `helm list -n crossplane-system`.

### AWS Provider

Now we can bind Crossplane with Localstack.  
We can install the AWS provider, that will install the controller and all the CRDs needed to manage the AWS resources

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: crossplane-provider-aws
spec:
  package: crossplane/provider-aws:v0.29.0
```

and we will need to create a secret containing the AWS credentials needed to connect to Localstack.

The `creds.conf` file should be something like this

```
[default]
aws_access_key_id = test
aws_secret_access_key = test
```

and to create the secret

```
$ kubectl create secret generic aws-creds -n crossplane-system --from-file=creds=creds.conf
```

And finally we can create the provider configuration that will point to our Localstack and that will use the just created secret:

```yaml
apiVersion: aws.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: aws-provider-config
spec:
  endpoint:
    url:
      type: Static
      static: http://172.18.0.4:4566
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: aws-creds
      key: creds
```
