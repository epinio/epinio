# Epinio supported applications

This section describes what kind of application you can expect to work with Epinio.
To understand what enables an application to work with Epinio, you need to know how staging works.


## How it works

Epinio relies on [Cloud Native Buildpacks](https://buildpacks.io/) to create a container image for your
application. It does that by installing and using [the upstream maintained Tekton pipelines](https://github.com/tektoncd/catalog/tree/main/task/buildpacks).

Staging starts with you (the developer) running `epinio push myapp` from the root of your application source code.
You can see a simplified diagram of the process in the image below:

![epinio-push-simplified](/docs/images/epinio-push-simple.svg?raw=true "Epinio push")

After pushing your code, Epinio triggers a Tekton pipeline which uses the [paketo buildpacks](https://paketo.io/) to build a runtime image for your application.
If you are not familiar with how buildpacks work, you should have a look at the official docs: https://buildpacks.io/docs/

## Supported buildpacks

Epinio uses the [full stack paketo builder image](https://github.com/paketo-buildpacks/full-stack-release) which means you can make use of any of the buildpacks
documented here: https://paketo.io/docs/buildpacks/language-family-buildpacks/

The various buildpacks provide various configuration options. You can read on how to generally configure a buildpack here: https://paketo.io/docs/buildpacks/configuration/
Each buildpack may support more configuration options, so you may have to read the documentation of the buildpacks you are interested in.

E.g. [Instructions on how to add custom php.ini files for php-web buildpack](https://github.com/paketo-buildpacks/php-web#configuring-custom-ini-files)

## Detailed push process

The above image is a simplified explanation of the `epinio push` process. If you don't want to know all the details on how that works, the above diagram should
be all the information you need. If you are curious about the details, then read here: [Detailed push docs](/docs/user/explanations/detailed-push-process.md)
