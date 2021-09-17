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

       - Create an analogous file `internal/applications/services.go` for managing the set of bound
         services.

       - Needs functions for

           - loading/creating the secret
           - adding service
           - removing service
           - replacing all services with new set
           - listing services

       - Counter to EV these function must not trigger a workload restart, when a workload is present.

         Reason: Part of the binding process, something which does not apply to EV, is the need to
         check at the time of integrating the bindings into the deployment if the service still
         exists.

         I.e. a service may be deleted between when it was bond to an app at rest, and when the
         application is deployed and actually tries to use the service.

         While this could be done in a new workload method doing so precludes the use of the APIerror
         interface for the collected errors. And having that, especially the Multierror is really
         nice, and desired.

      - __Reminder__ The new app service secret resource has to be owned by the app resource, to
        ensure that it partakes in the cascading deletion of an app.

      - `internal/application/workload.go`

        Rework the internals to apply all current services of an app in a single call (`ServicesChange`).
        Retire the `Bind`, `Unbind`, and `UnbindAll` functions.

        The relevant service operations `DeleteBinding`, `GetBinding` move into callers (i.e. API
        controllers). These are interface methods requiring a Service, i.e. just a name is not
        enough. This influences the API of the `ServicesChange`.

        `ServicesChange` shall take a `[]Service`, not `[]string`. Needed translation from service
        name to Service is in the caller. That also matches the callers validating the existence of
        the services to attach to the deployment to be.

        This means that service binding resources and associated secrets are created late, i.e. just
        before the service is used the first time by an app.

        See the EV handling for the general structure of `ServicesChange`.

      - Modify the `GetBinding` function of catalog based services to make binding resource (and
        secret?) owned by the app resource, to partake in the automatic deletion cascade.

  2. API server

       1. `internal/api/v1/servicebindings.go` holds the binding controller.
       
          Extend the code to manage the new app service secret using the new functionality from (1)
          above.

          The underlying un/bind operations are replaced by a single change operation simply applying
          the current state to a deployment. See last sub point in previous point.

       2. `internal/api/v1/services.go`

          Extend/modify the code for `Delete` to handle the app's service secret (see regular Unbind).
          Look into possibility of refactoring so that code can be shared with unbind itself.

          Modify `servicesToApps` to locate services by the app service secret instead of querying the
          workload.

          Generally check where services are handled only by workload if it can/has to be converted to
          the usage of the app service secret.

       3. `internal/api/v1/application.go`

          Extend `Create` and underlying request structure to take service names. Place them into the
          new app service secret.

          Extend the `AppUpdate` request structure to take service names also.

          This can reuse work from the aborted first attempt at this.

          Extend/modify `Deploy` to take service information from the app service secret and bind them
          to the application. See if we can use a mechanism analogous to EV for inserting things
          directly into the deployment instead of having to restart

    3. Client

         1. `internal/cli/clients/client.go`

            Modify `Push` to call `AppUpdate` when create fails.
            Remove the client-side bind call for services.
            These are all entered through app create or update.

            Move handling of `--bind` option into shared file, for use by both `push` and `create`.

         2. `internal/cli/clients/apps_create.go`
         
            Rework `AppCreate` to take `--bind`.

       Note that these changes can reuse work from the aborted first attempt.
