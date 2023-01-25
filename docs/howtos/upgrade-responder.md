# Upgrade Responder

The [Longhorn Upgrade Responder](https://github.com/longhorn/upgrade-responder) can be used to check when a new version of your application is available, and it can be used to collect some information about the client who made the request.

The Epinio dashboard is available at https://version.rancher.io.  

After the login you can find it under the `General` folder, or [here](https://version.rancher.io/d/Epinio-1/epinio-upgrade-responder).

## Local installation

To deploy a development instance of the Upgrade Responder you can run `make install-upgrade-responder` (and `make uninstall-upgrade-responder` to remove it).


The upgrade responder will be configured using the Epinio Releases with this command:

```bash
curl -s https://api.github.com/repos/epinio/epinio/releases | \
  jq '.[] | {
    Name: (.name | split(" ")[0]),
    ReleaseDate: .published_at,
    MinUpgradableVersion: "",
    Tags: [ .tag_name ],
    ExtraInfo: null
  }' | \
  jq -n '. |= [inputs]' | \
  jq '(first | .Tags) |= .+ ["latest"] | { 
    versions: .,
    requestIntervalInMinutes: 5
  }'
```

It will also setup the InfluxDB with the new parameters.

Please be aware that the queries and the configuration are set to use a 5m interval, while in production the interval should be higher (60m in the config, and 1h in the InfluxDB queries).

By default the local development version of Epinio has the tracking disabled, but the script will patch the server version enabling it and setting the local upgrade-responder address:

```
kubectl patch deployments -n epinio epinio-server --type=json --patch \
'[
  {"op": "add", "path": "/spec/template/spec/containers/0/env/-", "value": {"name": "UPGRADE_RESPONDER_ADDRESS", "value": "http://upgrade-responder:8314/v1/checkupgrade"}},
  {"op": "add", "path": "/spec/template/spec/containers/0/env/-", "value": {"name": "DISABLE_TRACKING", "value": "false"}}
]'
```
