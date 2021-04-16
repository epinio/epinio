# Epinio push in detail

Epinio strives to make use of well supported, well known and loved projects instead of re-inventing the wheel ([link](README.md#guidelines-soft-principles)).
But while doing so, it makes sure those components are deployed correctly and work together seamlessly. Let's go through the `epinio push` process in detail
so you can understand what each components does.

You can see an image that visualises the process lower in this page. Refer to it while reading the text to help you understand the process more.

## 1. Uploading the code

One of the components Epinio installs on your cluster is [Gitea](https://gitea.io/en-us/). Gitea is an Open Source code hosting solution. Among other things it allows
us to create repositories and organizations using API calls. It also used to store your application's code using which Epinio pushes using [`git`](https://git-scm.com/).

So the first thing Epinio does when you push your applicatio for the first time is to create a new project on Gitea and by using `git` to push your code there.
This doesn't mean you should be using `git` yourself. Epinio will create a tmp directory which will be the local git repository, copy your code over and then
commit all the local changes you may have (even if you haven't commited those yet on your own git branch).
Then it will push your code to Gitea.

## 2. Trigger of the Tekton pipeline

Now Gitea has your code but we need to tell buildpacks to build an image for your application. The buildpacks community maintains
[a set of Tekton pipeline definitions](https://github.com/tektoncd/catalog/tree/main/task/buildpacks) that can be used to do exactly that.
All we need to do is to tell the pipeline [where to find the application source](https://github.com/epinio/epinio/blob/bdcb1833829d58c5bdcf0e8ecb1998d766e72a13/embedded-files/tekton/triggers.yaml#L36-L44)
and then trigger the pipeline to start.

Triggering happens using a [TriggerBinding](https://github.com/tektoncd/triggers/blob/main/docs/triggerbindings.md) to which Gitea will make a request.
When you pushed your application in the previous step, Epinio set up a webhook on the Gitea project that makes a call to the trigger binding endpoint whenever
the code is pushed to Gitea.

So after you push your code, the above will ensure buildpacks will create an image for your application. You can see the logs of the staging process
either by pushing the app with the `--verbosity 1` flag set, or by looking at the logs of the staging pods that appear in your cluster.

So an image is generated, but where is it stored? Read on.

## 3. The registry

Epinio installs a container registry inside your cluster and that is where the application images are stored. This was installed when you first did `epinio install` and is
used to make the setup easier (by not having to configure an external registry) and staging faster (by keeping all image transferring local to the cluster).
There is not much to tell about it but if you want to look on how the registry is installed, have a look at the helm chart here:
https://github.com/epinio/epinio/tree/main/assets/container-registry/chart/container-registry

## 4. Creation of the Application Kubernetes Resources

To run a workload on Kubernetes having a container image is not enough. You need at least a Pod running with at least one container running that image.
The Tekton pipeline that created the image also created a [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/) for your application.
The deployment is set to use the generated image.

That is enough to get a Pod running but you still don't have access to your application from outside the cluster. When you installed Epinio, it created and
registered a [Kubernetes webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) which monitors the workloads namespace for new apps.
When a new application appears, it creates a [Service](https://kubernetes.io/docs/concepts/services-networking/service/) and an [Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) for it.
You can read how these work in Kubernetes following the provided links but if you have to know one thing is that Ingress is the thing that describes how a request that uses the Application's url is routed to the application
container. In Kubernetes, the thing that reads that description and implements the routing is called and Ingress Controller. Such an Ingress Controller is provided by [Traefik](https://doc.traefik.io/traefik/providers/kubernetes-ingress/).

## 5. Ingress implementation (Traefik)

When you installed Epinio, it looked on your cluster to see if you had [Traefik](https://doc.traefik.io/traefik/providers/kubernetes-ingress/) running. If it wasn't there it installed it. Traefik among other things, it an Ingress Controller. As explained above, the Ingress Controller reads your Ingress Resource Definitions and implements the desired routing to the appropriate Services/Pods.

In Epinio, for every application we create an Ingress that routes the traffic to you application through an subdomain that looks something like this:

```
myapplication.my_epinio_system_domain.com
```

You can get the route of your application with `epinio apps list` or `epinio apps show myapplication`

## 6. Additional things

During installation, if you specified a system domain using the `--system-domain` parameter, then your application routes will be sudomains of that domain.
Epinio considers this domain to be a production server and thus creates a production level TLS certificate for your application using [Let's Encrypt](https://letsencrypt.org/).
This happens using the [cert-manager](https://cert-manager.io/docs/installation/kubernetes/) which is one more of the components Epinio installs with `epinio install`.

If you didn't specify a system domain then Epinio uses a "magic DNS" service running on `omg.howdoi.website` which is similar to [nip.io](https://nip.io/), and [xip.io](http://xip.io/).
These services resolve all subdomains of the root domain to the subdomain IP address. E.g. `1.2.3.4.omg.howdoi.website` simply resolves to `1.2.3.4`. They are useful when you don't have
a real domain but you still need a wildcard domain to create subdomains to. Depending on your setup, the IP address of the cluster which Epinio discovers automatically may not be accessible
by your browser and thus you may need to set the system domain when installing to use another IP. This is the case for example when you run a Kubernetes cluster with docker (e.g. [k3d](https://k3d.io/) or [kind](https://github.com/kubernetes-sigs/kind)) inside a VM (for example when using docker on Mac). Then the IP address which Epinio detects is the IP address of the docker container but that is not accessible from your host. You will need to bind the container's ports `80` and `443` to the VMs ports `80` and `443` and then use the VMs IP address instead.


## The process visualized

![epinio-push-detailed](/docs/images/epinio-push-detailed.svg?raw=true "Epinio push")

## Credits

- Icons from: https://materialdesignicons.com/ (Source: https://github.com/Templarian/MaterialDesign)

