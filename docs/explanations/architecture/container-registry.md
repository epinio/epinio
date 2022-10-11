# Configuration of the internal container registry

The internal container registry is the registry Epinio will be deployed with if the operator has not
specified everything needed to work with an external registry.

The current setup of this registry evolved under the following constraints:

  1. The paketo lifecycle creator used by our staging accesses the registry using a TLS-secured
  channel.

     It cannot be configured to use an unsecured channel.

     See [Paketo Ticket #524](https://github.com/buildpacks/lifecycle/issues/524)
     for a request to change this.

  2. For the application pods to use a TLS-secured channel for access to the registry it is necesssary to either

       a. "Let the Epinio user manually configure Kubernetes to trust the CA"
       b. "Use a well-known trusted CA, so there's no configuration needed"

     Quotes above from [Advanced Topics: Container Registry](https://docs.epinio.io/explanations/advanced#container-registry)
     Neither was desired, leaving the last option to "... not encrypt the communication at all".

  3. No way was found to configure the registry itself to allow both secured and unsecured access.

  4. See also [Chart PR #125](https://github.com/epinio/helm-charts/pull/125)

The result is shown below

<img src="./container-registry.svg" align="right">

The registry is exposed through two k8s services, one for direct secure access by staging, the other
for indirect unsecured access by application pods.

The second service uses NginX listening on a node port to accept unsecured connections and then
proxy them to the secured access.
