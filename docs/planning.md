
# Operational planning

  1. We have an additional representation for services bound to an app, a kube secret holding the
     service names as keys, with empty values.

     This is very similar to how EVs for apps are stored.

     The management for the EV secret is held in file `internal/applications/env.go`. It has functions
     for

       - loading/creating the secret
       - adding entries
       - removing entries
       - listing entries

     When changing the environment the functions determine if a workload is active and trigger a
     restart with the new variables.

     Plan:

       - __((done))__ Create an analogous file `internal/applications/services.go` for managing the set of bound
         services.

       - __((done))__ Needs functions for

           - loading/creating the secret
           - adding service
           - removing service
           - replacing all services with new set
           - listing services

       - __((done))__ Counter to EV these function must not trigger a workload restart, when a workload is present.

         Reason: Part of the binding process, something which does not apply to EV, is the need to check at the
         time of integrating the bindings into the deployment if the service still exists.

         I.e. a service may be deleted between when it was bond to an app at rest, and when the application is
         deployed and actually tries to use the service.

         While this could be done in a new workload method doing so precludes the use of the APIerror interface for
         the collected errors. And having that, especially the Multierror is really nice, and desired.

      - __((done))__ __Reminder__ The new app service secret resource has to be owned by the app resource, to
        ensure that it partakes in the cascading deletion of an app.

      - `internal/application/workload.go`

        __((done))__ Rework the internals to apply all current services of an app in a single call
        (`ServicesChange`).  Retire the `Bind`, `Unbind`, and `UnbindAll` functions.

        __((done))__ The relevant service operations `DeleteBinding`, `GetBinding` move into callers
        (i.e. API controllers). These are interface methods requiring a Service, i.e. just a name is
        not enough. This influences the API of the `ServicesChange`.

        __((done))__ `ServicesChange` shall take a `[]Service`, not `[]string`. Needed translation
        from service name to Service is in the caller. That also matches the callers validating the
        existence of the services to attach to the deployment to be.

        __((done))__ This means that service binding resources and associated secrets are created
        late, i.e. just before the service is used the first time by an app.

        See the EV handling for the general structure of `ServicesChange`.

      - __((done))__ Modify the `GetBinding` function of catalog based services to make binding resource owned by
        the app resource, to partake in the automatic deletion cascade. Note, the associated secret is owned by the
        binding resource already, no need for further ownership.

  2. API server

       1. `internal/api/v1/servicebindings.go` holds the binding controller.
       
          __((done))__ Extend the code to manage the new app service secret using the new
          functionality from (1) above.

          __((done))__ The underlying un/bind operations are replaced by a single change operation
          simply applying the current state to a deployment. See last sub point in previous point.

       2. `internal/api/v1/services.go`

          __((done))__ Extend/modify the code for `Delete` to handle the app's service secret (see
          regular Unbind).  Look into possibility of refactoring so that code can be shared with
          unbind itself.

          __((done))__ Modify `servicesToApps` to locate services by the app service secret instead of
          querying the workload.

          __((done))__ Generally check where services are handled only by workload if it can/has to be
          converted to the usage of the app service secret.

       3. `internal/api/v1/application.go`

          __((done))__ Extend/Modify `Index` to provide services, instances for apps at rest.

          __((done))__ Extend `Create` and underlying request structure to take service names.

          __((done))__ Extend the `AppUpdate` request structure to take service names also.

          __((done))__ Ditto for number of instances

          __((done))__ Place them into the new app service secret.

          __((done))__ Place them into the new app scaling secret.

          __((done))__ This can reuse work from the aborted first attempt at this.

          __((done))__ Extend/modify `Deploy` to take service information from the app service secret
          and bind them to the application. See if we can use a mechanism analogous to EV for
          inserting things directly into the deployment instead of having to restart

    3. Client

         1. `internal/cli/clients/client.go`

            __((done))__ Modify `Push` to call `AppUpdate` when create fails.

            __((done))__ Remove the client-side bind call for services.

            __((done))__ These are all entered through app create or update.

            __((done))__ Move handling of `--bind` option into shared file, for use by both `push` and `create`.

            __((done))__ Ditto for `--instances` (users: `app push`, `app update`, to come: `app create`).

            Extend/Modify `app list` to show desired instances of an app.

         2. `internal/cli/clients/apps_create.go`
         
            __((done))__ Rework `AppCreate` to take `--bind`.

       Note that these changes can reuse work from the aborted first attempt.

    4. __((done))__ Application tech debt

         - Remove Lookup/List
         - Create an app method to check for/get workload
         - Do lookup more directly, instead of via list
         - Refactor the stuff around `Complete()`.
         - Workload information in the workload structure! not app itself.
