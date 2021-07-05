### Get yourself a cluster

You may already have a Kubernetes cluster you want to use to deploy Epinio. If
not, you can create one with [k3d](https://k3d.io/). Follow the instructions on
[the k3d.io website](https://k3d.io/) to install k3d on your system. Then get
youself a cluster with the following command:

```bash
$ k3d cluster create epinio
```

After the command returns, `kubectl` should already be talking to your new cluster:

```bash
$ kubectl get nodes
NAME                  STATUS   ROLES                  AGE   VERSION
k3d-epinio-server-0   Ready    control-plane,master   38s   v1.20.0+k3s2
```

### Install dependencies

- `kubectl`: Follow instructions here: https://kubernetes.io/docs/tasks/tools/#kubectl
- `helm`: Follow instructions here: https://helm.sh/docs/intro/install/

### Install Epinio

Get the latest version of the binary that matches your Operating System here:
https://github.com/epinio/epinio/releases

Install it on your system and make sure it is in your `PATH` (or otherwise
available in your command line).

Now install Epinio on your cluster with this command:

```bash
$ epinio install
```

That's it! If everything worked as expected you are now ready to push your first
application to your Kubernetes cluster with Epinio.

In case of trouble with Epinio's Traefik component or Ingress controllers, the
[Traefik](docs/advanced.md#traefik) section in the
[Advanced Topics](docs/advanced.md) document shall be your friend.




### Push an application

__Note__: If you want to know the details of the `epinio push` process, please
read the [detailed push docs](/docs/detailed-push-process.md)

If you just want an application that works use the one inside the
[sample-app directory](assets/sample-app). You can copy it to your system with
the following commands:

```
$ git clone https://github.com/epinio/epinio.git
$ cd epinio/assets/
```

To push the application run the following command:

```bash
$ epinio push sample sample-app
```

where `sample` is the name you want to give to your application. This name has
to be unique within the targeted organization in Epinio. `sample-app` is path to
the directory where your application's code resides.

Note that the path argument is __optional__.
If not specified the __current working directory__ will be used.
Always ensure that the chosen directory contains a supported application.

If you want to know what applications are supported in Epinio, please read the
[notes about supported applications](/docs/supported-apps.md).

We also provide information about the more advanced
[git mode](docs/advanced.md#git-pushing).

### Check that your application is working

After the application has been pushed, a unique URL is printed which you can use
to access your application. If you don't have this URL available anymore you can
find it again by running:

```bash
$ epinio app show sample
```

("Routes" is the part your are looking for)

Go ahead and open the application route in your browser!

### List all commands

To see all the applications you have deployed use the following command:

```bash
$ epinio apps list
```

### Delete an application

To delete the application you just deployed run the following command:

```bash
$ epinio delete sample
```

### Create a separate org

If you want to keep your various application separated, you can use the concept
of orgs (aka organizations). Create a new organization with this command:

```bash
$ epinio orgs create neworg
```

To start deploying application to this new organization you need to "target" it:


```bash
$ epinio target neworg
```

After this and until you target another organization, whenever you run `epinio push`
you will be deploying to this new organization.
