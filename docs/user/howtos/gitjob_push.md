One of the more interesting features of Rio was that it would let you set up a deployment that rebuilds and republishes when your code stored in Git is changed. 

We can recreate this functionality using the GitJob CRD that's a part of [Rancher Fleet](https://fleet.rancher.io/).

NOTE: We will improve this experience in the future!

## Setup

### Install GitJob

If you don't have Rancher (or standalone Fleet) installed, we need to install the GitJob operator by following the isntructions found at https://github.com/rancher/gitjob#running.


Then we need to setup the Service Account to run our Jobs with (since we don't need to do anything directly with the kube api, we don't need to add any role bindings to it):

```
apiVersion: v1
kind: ServiceAccount
metadata:
  name: epinio-push
```

### Upload Epinio Config

So the GitJob can authenticate and push correctly, we can upload our Epinio config file to the cluster with:

```
kubectl create secret generic epinio-creds --from-file=$HOME/.config/epinio/config.yaml
```

This will create a secret containing the config.yaml that was created locally when you do `epinio install` or `epinio config update`

### Setup Sample Project

Next, we can use the 12factor app to show how to write a GitJob.

Create a yaml file called `12factor-gitjob.yaml` containing:

``` yaml
apiVersion: gitjob.cattle.io/v1
kind: GitJob
metadata:
  # The name of the GitJob, doesn't need to match the project.
  name: samplepush
spec:
  syncInterval: 15
  git:
    # The git repo and branch to track. 
    repo: https://github.com/scf-samples/12factor
    branch: scf
  jobSpec:
    template:
      spec:
        # This should match what we created in the last step
        serviceAccountName: epinio-gitjob
        restartPolicy: "Never"
        containers:
        # This version should match your epinio deployment
        - image: "splatform/epinio-server:v0.1.0"
          name: epinio-push
          volumeMounts:
          - name: config
            mountPath: "/config/"
            readOnly: true
          env:
          - name: EPINIO_CONFIG
            value: "/config/config.yaml"
          command:
          - /epinio 
          args:
          - push 
          # This is the name of the app to push
          - test12factor
          workingDir: /workspace/source
        volumes:
        - name: config
          secret:
            secretName: epinio-creds
```


You can apply this via:

```
kubectl apply -f 12factor-gitjob.yaml
```

Once applied, you should see a Job and then Pod get created:

```
kubectl get job,pod
```

You can follow the logs of the pod listed above with:

```
kubectl logs <pod_name> -f
```


### Using Webhooks

If you prefer to use webhooks instead of polling, set up the job in the same way as before but also follow the instructions found at: https://github.com/rancher/gitjob#webhook

