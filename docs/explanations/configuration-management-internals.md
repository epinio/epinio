# Configuration Management

## Representation

Configurations bound to an application are stored in a kube secret,
analogously to how the environment variables of an application are
stored. Each configuration is represented by a key of the secret, holding
the name of the configuration. This automatically ensures that it is not
possible to have duplicate configuration bindings.

Contrary to EVs the value of each key is left empty. They do not
matter.

Then, when an application is deployed the named configurations are stored in
the application's deployment, as volumes referencing the configurations'
binding secret resources.

__Note__: The configuration binding resources and associated secrets of
__catalog-based__ configurations are owned by the app resource, as they are
tied to the app (Materialization of the n:m relation between
applications and configurations). This makes removal on app deletion easier,
as it will happen automatically as part of the cascade taking down
everything associated with an application.

The bindings of custom configurations are not treated this way, as these are
shared between applications, and thus cannot be owned by a single
one. Unbinding them is not deleting anything either also.

## Commands

  - `configuration bind S A`
  - `configuration unbind S A`
  - `configuration delete S`
  - `app create --bind S,... ... A`
  - `app push --bind S,... ... --name A ...`
  - `app delete A`

### Semantics: `configuration bind S A`

The named configuration `S is bound to the named application `A`.

To this end the application's configuration secret is pulled, modified, and
written back with the configuration added as new key.

If the application is active, then the binding is further applied to
the application's workload, a kubernetes `Deployment`. This change
forces the deployment to restart the application's pods.

The relevant API endpoint is `ConfigurationBindingCreate`
(`POST /namespaces/:namespace/applications/:app/configurationbindings`)

TODO: replace by equivalent SVG graphic
```
user --> client  :configuration bind S A
         client --> server  :POST ConfigurationBindingCreate O A (S)
                    server --> cluster :validate O
                    server <-- cluster :ok/fail
                    server --> cluster :validate A
                    server <-- cluster :ok/fail
                    //
                    server --> cluster :read A's configuration secret
                    server <-- cluster :ok/fail
                    server --> cluster :modify A's configuration secret (add S)
                    server <-- cluster :ok/fail
                    server --> cluster :write A's configuration secret
                    server <-- cluster :ok/fail
                    //
                    server --> cluster :validate workload A, skip following if missing
                    server <-- cluster :ok/fail
                    server --> cluster :get deployment A
                    server <-- cluster :deployment
                    server --> cluster :modify deployment (add configuration secret (as) volume)
                    server <-- cluster :ok/fail
         client <-- server  :report ok/fail
user <-- client  :report ok/fail
```

### Semantics: `configuration unbind S A`

The named configuration `S is unbound from the named application `A`.

To this end the application's configuration secret is pulled, modified, and
written back with the configuration's key removed from it.

If the application is active, then the binding is further undone in
the application's workload, a kubernetes `Deployment`. This change
forces the deployment to restart the application's pods.

The relevant API endpoint is `ConfigurationBindingDelete`
(`DELETE /namespaces/:namespace/applications/:app/configurationbindings/:configuration`)

TODO: replace by equivalent SVG graphic
```
user --> client  :configuration unbind S A
         client --> server  :DELETE ConfigurationBindingDelete O A S
                    server --> cluster :validate O
                    server <-- cluster :ok/fail
                    server --> cluster :validate A
                    server <-- cluster :ok/fail
                    //
                    server --> cluster :read A's configuration secret
                    server <-- cluster :ok/fail
                    server --> cluster :modify A's configuration secret (remove S)
                    server <-- cluster :ok/fail
                    server --> cluster :write A's configuration secret
                    server <-- cluster :ok/fail
                    //
                    server --> cluster :validate workload A, skip following if missing
                    server <-- cluster :ok/fail
                    server --> cluster :get deployment A
                    server <-- cluster :deployment
                    server --> cluster :modify deployment (remove configuration secret volume)
                    server <-- cluster :ok/fail
                    server --> cluster :remove configuration binding resource
                    server <-- cluster :ok/fail
         client <-- server  :report ok/fail
user <-- client  :report ok/fail
```

### Semantics: `configuration delete S`

Deletes the named configuration `S`.

By default this action is rejected when `S` is still bound to one or
more applications.

Specification of the `--unbind` option forces the command to unbind
`S` from all applications `A` using it and then deleting `S`.

All touched applications are restarted.

TODO: replace by equivalent SVG graphic
```
user --> client  :configuration delete S ?--unbind?
         client --> server  :DELETE ConfigurationDelete O S (unbind)
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

The named configurations S... are bound to the named application `A`, newly
created by the command as well.

As the newly created application is not active, without workload only
the application's configuration secret is modified.

Everything is performed on the server side, the client issues only an
`AppCreate` call containing all the necessary information.

TODO: replace by equivalent SVG graphic
```
user --> client  :configuration bind S A
         client --> server :POST AppCreate O (A, S...)
                    server --> cluster :create app resource
                    server <-- cluster :ok/fail
                    //
                    server --> cluster :read A's configuration secret
                    server <-- cluster :ok/fail
                    server --> cluster :modify A's configuration secret (add S...)
                    server <-- cluster :ok/fail
                    server --> cluster :write A's configuration secret
                    server <-- cluster :ok/fail
                    //
         client <-- server  :report ok/fail
user <-- client  :report ok/fail
```

### Semantics: `app push --bind S,... ... --name A ...`

The named configurations S... are bound to the named application `A`.

For a newly created application the configurations are bound via the
`AppCreate` API call, and the bindings are then picked up by the
deployment stage, for integration into the application's deployment
resource.

In the case of an already existing application the creation will fail
and trigger a call to `AppUpdate` instead which updates the
application's configuration resource with the new configurations.

__Attention__ Note that the above does __not__ re-start the
application. The new configurations apply only to the new revision of the
application, and not the currently running revision.

Integration of the new configurations happens then as for a new application,
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
                    server --> cluster :read A's configuration secret
                    server <-- cluster :ok/fail
                    server --> cluster :modify A's configuration secret (set S...)
                    server <-- cluster :ok/fail
                    server --> cluster :write A's configuration secret
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

This action automatically unbinds all configurations `S` bound to `A`.

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

		    // Nothing to be done for bound configurations. Removed as part of the cascade

                    server --> cluster :remove app resource
                    server <-- cluster :ok/fail
                    // cascades: deployment, pods, ingress, configuration, EV secret, Configuration secret, bindings

         client <-- server :ok/fail
user <-- client :report ok fail
```

## API endpoints

|Name                 |Op     |Location                                                      |
|---                  |---    |---                                                           |
|AppCreate            |POST   |`/namespaces/:namespace/applications`                               |
|AppDelete            |DELETE |`/namespaces/:namespace/applications/:app`                          |
|AppUpdate            |PATCH  |`/namespaces/:namespace/applications/:app`                          |
|ConfigurationBindingCreate |POST   |`/namespaces/:namespace/applications/:app/configurationbindings`          |
|ConfigurationBindingDelete |DELETE |`/namespaces/:namespace/applications/:app/configurationbindings/:configuration` |
|ConfigurationDelete        |DELETE |`/namespaces/:namespace/configurations/:configuration`                          |
