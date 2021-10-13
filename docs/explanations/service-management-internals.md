# Service Management

## Representation

Services bound to an application are stored in a kube secret,
analogously to how the environment variables of an application are
stored. Each service is represented by a key of the secret, holding
the name of the service. This automatically ensures that it is not
possible to have duplicate service bindings.

Contrary to EVs the value of each key is left empty. They do not
matter.

Then, when an application is deployed the named services are stored in
the application's deployment, as volumes referencing the services'
binding secret resources.

__Note__: The service binding resources and associated secrets of
__catalog-based__ services are owned by the app resource, as they are
tied to the app (Materialization of the n:m relation between
applications and services). This makes removal on app deletion easier,
as it will happen automatically as part of the cascade taking down
everything associated with an application.

The bindings of custom services are not treated this way, as these are
shared between applications, and thus cannot be owned by a single
one. Unbinding them is not deleting anything either also.

## Commands

  - `service bind S A`
  - `service unbind S A`
  - `service delete S`
  - `app create --bind S,... ... A`
  - `app push --bind S,... ... --name A ...`
  - `app delete A`

### Semantics: `service bind S A`

The named service `S is bound to the named application `A`.

To this end the application's service secret is pulled, modified, and
written back with the service added as new key.

If the application is active, then the binding is further applied to
the application's workload, a kubernetes `Deployment`. This change
forces the deployment to restart the application's pods.

The relevant API endpoint is `ServiceBindingCreate`
(`POST /namespaces/:org/applications/:app/servicebindings`)

TODO: replace by equivalent SVG graphic
```
user --> client  :service bind S A
         client --> server  :POST ServiceBindingCreate O A (S)
                    server --> cluster :validate O
                    server <-- cluster :ok/fail
                    server --> cluster :validate A
                    server <-- cluster :ok/fail
                    //
                    server --> cluster :read A's service secret
                    server <-- cluster :ok/fail
                    server --> cluster :modify A's service secret (add S)
                    server <-- cluster :ok/fail
                    server --> cluster :write A's service secret
                    server <-- cluster :ok/fail
                    //
                    server --> cluster :validate workload A, skip following if missing
                    server <-- cluster :ok/fail
                    server --> cluster :get deployment A
                    server <-- cluster :deployment
                    server --> cluster :modify deployment (add service secret (as) volume)
                    server <-- cluster :ok/fail
         client <-- server  :report ok/fail
user <-- client  :report ok/fail
```

### Semantics: `service unbind S A`

The named service `S is unbound from the named application `A`.

To this end the application's service secret is pulled, modified, and
written back with the service's key removed from it.

If the application is active, then the binding is further undone in
the application's workload, a kubernetes `Deployment`. This change
forces the deployment to restart the application's pods.

The relevant API endpoint is `ServiceBindingDelete`
(`DELETE /namespaces/:org/applications/:app/servicebindings/:service`)

TODO: replace by equivalent SVG graphic
```
user --> client  :service unbind S A
         client --> server  :DELETE ServiceBindingDelete O A S
                    server --> cluster :validate O
                    server <-- cluster :ok/fail
                    server --> cluster :validate A
                    server <-- cluster :ok/fail
                    //
                    server --> cluster :read A's service secret
                    server <-- cluster :ok/fail
                    server --> cluster :modify A's service secret (remove S)
                    server <-- cluster :ok/fail
                    server --> cluster :write A's service secret
                    server <-- cluster :ok/fail
                    //
                    server --> cluster :validate workload A, skip following if missing
                    server <-- cluster :ok/fail
                    server --> cluster :get deployment A
                    server <-- cluster :deployment
                    server --> cluster :modify deployment (remove service secret volume)
                    server <-- cluster :ok/fail
                    server --> cluster :remove service binding resource
                    server <-- cluster :ok/fail
         client <-- server  :report ok/fail
user <-- client  :report ok/fail
```

### Semantics: `service delete S`

Deletes the named service `S`.

By default this action is rejected when `S` is still bound to one or
more applications.

Specification of the `--unbind` option forces the command to unbind
`S` from all applications `A` using it and then deleting `S`.

All touched applications are restarted.

TODO: replace by equivalent SVG graphic
```
user --> client  :service delete S ?--unbind?
         client --> server  :DELETE ServiceDelete O S (unbind)
                    server --> cluster :validate O
                    server <-- cluster :ok/fail
                    server --> cluster :validate S
                    server <-- cluster :ok/fail
                    // if unbind
                    server --> cluster :determine the A's S is bound to
                    server <-- cluster :ok/fail
                    // per A
                    ((unbind sequence))
                    // end loop
                    // end if
         client <-- server  :report ok/fail
user <-- client  :report ok/fail
```

### Semantics: `app create --bind S,... ... A`

The named services S... are bound to the named application `A`, newly
created by the command as well.

As the newly created application is not active, without workload only
the application's service secret is modified.

Everything is performed on the server side, the client issues only an
`AppCreate` call containing all the necessary information.

TODO: replace by equivalent SVG graphic
```
user --> client  :service bind S A
         client --> server :POST AppCreate O (A, S...)
                    server --> cluster :create app resource
                    server <-- cluster :ok/fail
                    //
                    server --> cluster :read A's service secret
                    server <-- cluster :ok/fail
                    server --> cluster :modify A's service secret (add S...)
                    server <-- cluster :ok/fail
                    server --> cluster :write A's service secret
                    server <-- cluster :ok/fail
                    //
         client <-- server  :report ok/fail
user <-- client  :report ok/fail
```

### Semantics: `app push --bind S,... ... --name A ...`

The named services S... are bound to the named application `A`.

For a newly created application the services are bound via the
`AppCreate` API call, and the bindings are then picked up by the
deployment stage, for integration into the application's deployment
resource.

In the case of an already existing application the creation will fail
and trigger a call to `AppUpdate` instead which updates the
application's service resource with the new services.

__Attention__ Note that the above does __not__ re-start the
application. The new services apply only to the new revision of the
application, and not the currently running revision.

Integration of the new services happens then as for a new application,
as part of its deployment, after re-staging.

TODO: replace by equivalent SVG graphic
```
user --> client :app push ... -b S ... -n A
         client --> server :POST AppCreate O (A, S...)
                    server --> cluster :create app resource
                    server <-- cluster :ok/fail
         client <-- server :ok/fail

	 // if create failed
         client --> server :POST AppUpdate O A (S...)
                    server --> cluster :modify app resource     /(instances?!)
                    server <-- cluster :ok/fail
                    //
                    server --> cluster :read A's service secret
                    server <-- cluster :ok/fail
                    server --> cluster :modify A's service secret (set S...)
                    server <-- cluster :ok/fail
                    server --> cluster :write A's service secret
                    server <-- cluster :ok/fail
         client <-- server :ok/fail
	 // endif

         ((upload sources))
         ((stage sources))
         ((deploy image (stage result)))
                server side -- per S: ((bind sequencing))

user <-- client :report ok fail
```

### Semantics: `app delete A`

Deletes the named application `A`.

This action automatically unbinds all services `S` bound to `A`.

TODO: replace by equivalent SVG graphic
```
user --> client :app delete A
         client --> server :DELETE AppDelete O A
                    server --> cluster :validate O
                    server <-- cluster :ok/fail
                    server --> cluster :validate A
                    server <-- cluster :ok/fail

                    server --> cluster :remove app workload/deployment
                    server <-- cluster :ok/fail

		    // Nothing to be done for bound services. Removed as part of the cascade

                    server --> cluster :remove app resource
                    server <-- cluster :ok/fail
                    // cascades: deployment, pods, ingress, service, EV secret, Service secret, bindings

         client <-- server :ok/fail
user <-- client :report ok fail
```

## API endpoints

|Name                 |Op     |Location                                                      |
|---                  |---    |---                                                           |
|AppCreate            |POST   |`/namespaces/:org/applications`                               |
|AppDelete            |DELETE |`/namespaces/:org/applications/:app`                          |
|AppUpdate            |PATCH  |`/namespaces/:org/applications/:app`                          |
|ServiceBindingCreate |POST   |`/namespaces/:org/applications/:app/servicebindings`          |
|ServiceBindingDelete |DELETE |`/namespaces/:org/applications/:app/servicebindings/:service` |
|ServiceDelete        |DELETE |`/namespaces/:org/services/:service`                          |
