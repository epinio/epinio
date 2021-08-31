# QuickStart 

If you have not already installed `epinio` follow these links

- [Install on K3d](./install_epinio_on_k3d.md)

In this tutorial, you will learn how to create a namespace and how to push, list and delete an application in it.

## Push an application

### Clone the sample app

If you just want an application that works use the one inside the
[sample-app directory](/assets/sample-app). You can copy it to your system with
the following commands:

```bash
git clone https://github.com/epinio/epinio.git
cd epinio/assets/
```

### Push an app

```bash
epinio push sample sample-app
```

where `sample` is the name you want to give to your application. This name has to be unique within the targeted namespace in Epinio. `sample-app` is path to the directory where your application's code resides.

Note that the path argument is __optional__. If not specified the __current working directory__ will be used. Always ensure that the chosen directory contains a supported application.

If you want to know what applications are supported in Epinio, please read the
[notes about supported applications](../references/supported-apps.md).

We also provide information about the more advanced [git model](../explanations/advanced.md#git-pushing).

__Note__: If you want to know the details of the `epinio push` process, please read the [detailed push docs](../explanations/detailed-push-process.md)


#### Check that your application is working

After the application has been pushed, a unique URL is printed which you can use to access your application. If you don't have this URL available anymore you can find it again by running:

```bash
epinio app show sample
```

("Routes" is the part your are looking for)

Go ahead and open the application route in your browser!

### List all commands

To see all the applications you have deployed use the following command:

```bash
epinio apps list
```

### Delete an application

To delete the application you just deployed run the following command:

```bash
epinio delete sample
```

### Create a separate namespace

If you want to keep your various application separated, you can use the concept of namespaces. Create a new namespace with this command:

```bash
epinio namespace create newspace
```

To start deploying application to this new namespace you have to "target" it:


```bash
epinio target newspace
```

After this and until you target another namespace, whenever you run `epinio push` you will be deploying to this new namespace.
