# Epinio push in detail

Epinio strives to make use of well supported, well known and loved projects instead of re-inventing the wheel ([link](README.md#guidelines-soft-principles)).
But while doing so, it makes sure those components are deployed correctly and work together seamlessly. Let's go through the `epinio push` process in detail
so you can understand what each components does.

## 1. Uploading the code

One of the components Epinio installs on your cluster is [Gitea](https://gitea.io/en-us/). Gitea is an Open Source code hosting solution. Among other things it allows
us to create repositories and organizations using API calls. It also used to store your application's code using which Epinio pushes using [`git`](https://git-scm.com/).

So the first thing Epinio does when you push your applicatio for the first time is to create a new project on Gitea and by using `git` to push your code there.
This doesn't mean you should be using `git` yourself. Epinio will create a tmp directory which will be the local git repository, copy your code over and then
commit all the local changes you may have (even if you haven't commited those yet on your own git branch).
Then it will push your code to Gitea.

## 2. Trigger of the Tekton pipeline

## 3. Creation of the Application Deployment

## 4. Creation of the Application Service and Ingress

## 5. Ingress implementation (Traefik)

## 6. Special cases

## Credits

- Icons from: https://materialdesignicons.com/ (Source: https://github.com/Templarian/MaterialDesign)

