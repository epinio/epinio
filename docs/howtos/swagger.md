# Swagger

A Swagger endpoint is available on `/api/swagger.json`. This will expose the OpenAPI spec for the Epinio API.

## Generate the `swagger.json` file

To generate the `swagger.json` file you need `github.com/go-swagger/go-swagger`.

You can just run the `make swagger` command and it will install the tool if needed, and generate the `swagger.json` file.


## Serve the documentation

### Development mode

To see the documentation of a local deployment you can run the `make swagger-serve` command.
It will query your running Epinio for its swagger file and you will be able to try out the APis.

### External Epinio instance

If you want run the documentation on a running Epinio deployment you just need to specify the URL:

```
docker run -p 8080:8080 -e SWAGGER_JSON_URL=https://epinio.<EPINIO_SYSTEM_DOMAIN>/api/swagger.json swaggerapi/swagger-ui
```
but remember that since you are running the documentation locally then you would need to enable CORS on your deployment.
You can do it specifying the `server.accessControlAllowOrigin: "*"` value in the Helm values.

### Locally

If you just want to see the documentation without a running instance of Epinio you can do it with:

```
docker run -p 8080:8080 -e SWAGGER_JSON=/docs/swagger.json -v $(pwd)/docs/references/api:/docs swaggerapi/swagger-ui
```
