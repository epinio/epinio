Installs in less than 1 minute.
After install, uses ~ 65 mb of memory.


Unfortunately the following instructions are GKE specific!

```
kubectl create clusterrolebinding cluster-admin-binding \
  --clusterrole cluster-admin \
  --user $(gcloud config get-value account)

kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v0.40.2/deploy/static/provider/cloud/deploy.yaml

```

> See https://kubernetes.github.io/ingress-nginx/deploy/