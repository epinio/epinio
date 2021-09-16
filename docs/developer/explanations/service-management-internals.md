# Service Management

## Representation

Services bound to an application are stored in the application's
deployment, as volumes referencing the services' binding secret
resources.

## Commands

  - `service bind S A`
  - `service unbind S A`
  - `service delete S`
  - `app push --bind S,... ... A`
  - `app delete A`

### Semantics: `service bind S A`

The named service `S is bound to the named __active__ application `A`.

The binding is actually done to the application's workload, a
kubernetes `Deployment`. This change forces the deployment to restart
the application's pods.

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

The named service `S is unbound from the named __active__ application `A`.

The unbinding is actually done to the application's workload, a
kubernetes `Deployment`. This change forces the deployment to restart
the application's pods.

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
more applications. The application are __active___, by definition.

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

### Semantics: `app push --bind S,... ... A`

The named services S... are bound to the named __active__ application `A`.

This is done using the same process as done by `service bind S A`, __after__
the application is created, staged, and deployed, i.e created and made active..

__Note__, if the application already exists the creation step fails
with a conflict. This failure is ignored. The effective process is as
above, without creation.

TODO: replace by equivalent SVG graphic
```
user --> client :app push ... -b S ... A
         client --> server :POST AppCreate O (A)
                    server --> cluster :create app resource
                    server <-- cluster :ok/fail
         client <-- server :ok/fail

         ((upload sources))
         ((stage sources))
         ((deploy image (stage result)))

         per S: ((bind sequencing))

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

                    // per bound S
                    ((unbind sequence)) - for the binding secrets only
		    // NOTE: make them owned by the app resource! See cascade below

                    server --> cluster :remove app resource
                    server <-- cluster :ok/fail
                    // cascades: deployment, pods, ingress, service, EV secret

         client <-- server :ok/fail
user <-- client :report ok fail
```

## API endpoints

|Name                 |Op     |Location                                                      |
|---                  |---    |---                                                           |
|AppCreate            |POST   |`/namespaces/:org/applications`                               |
|AppDelete            |DELETE |`/namespaces/:org/applications/:app`                          |
|ServiceBindingCreate |POST   |`/namespaces/:org/applications/:app/servicebindings`          |
|ServiceBindingDelete |DELETE |`/namespaces/:org/applications/:app/servicebindings/:service` |
|ServiceDelete        |DELETE |`/namespaces/:org/services/:service`                          |
