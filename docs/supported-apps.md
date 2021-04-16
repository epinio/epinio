# Epinio supported applications

This sections describes what kind of application you could expect to work with Epinio.
To understand what makes an application to work with Epinio you need to know how staging works.


## How it works

Epinio relies on [Cloud Native Buildpacks](https://buildpacks.io/) to create a container image for your
application. It does that by installing and using [the upstream maintained Tekton pipelines](https://github.com/tektoncd/catalog/tree/main/task/buildpacks).

Staging starts with you (the developer) running `epinio push myapp` from the root of your application source code.
You can see a simplified diagram of the process in the image below:

![epinio-push-simplified](/docs/images/epinio-push-simple.svg?raw=true "Epinio push")

## Credits

- Icons from: https://materialdesignicons.com/ (Source: https://github.com/Templarian/MaterialDesign)

