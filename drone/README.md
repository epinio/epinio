## Drone

Installs in less than 30 sec.
After install, uses less than 20 mb of memory.
Image size 68 mb.


```
helm repo add drone https://charts.drone.io
kubectl create ns drone
helm install drone drone/drone --namespace drone --values drone-values.yaml
```

## Runner

Installs in less than 30 sec.
Less than 20 mb of memory.
Image size 50 mb.

Made it `cluster-admin` so it can create namespaces.

```
helm install drone-runner-kube drone/drone-runner-kube --namespace drone --values runner-values.yaml
kubectl apply -f drone-runner-service-account.yaml 
```
