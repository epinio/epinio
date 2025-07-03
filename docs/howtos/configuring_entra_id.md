# Configuring Microsoft Entra ID SSO

In order to configure SSO using Microsoft Entra ID, we need to set up a client within Azure and configure Dex with that client's details.  This document will attempt to provide instructions as concisely as possible.  Here is the general outline of required steps:

1. Create App Registration
2. Create Client Secret
3. Grant Directory Permissions
4. Add Connector to Dex Configuration

Once completed, your Epinio instance should now allow signin through Entra ID instead of local user authentication.


## Create App Registration

1. Visit Microsoft Entra ID
2. Note the `Tenant ID`
3. Navigate to **App Registrations**
4. Click **New Registration**
5. Configure the new registration
	- The name is arbitrary, e.g. `epinio-sso`
	- Single Tenant
	- Set the platform as `Web`
	- Set the Redirect URI as `https://auth.<your-domain>/callback`
6. Click **Register**
7. Note the `Client ID`

We should now have a `Client ID` and `Tenant ID` which will be used later to configure our Dex connector.

## Create Client Secret

1. Navigate to your new app registration
2. Navigate to that app registration's **Certificates & secrets**
3. Click **New Client Secret**
4. Set an arbitrary description and expiration
5. Click **Add**
6. Copy the **Secret Value** for your newly added client secret

We should now have a `Client Secret` which will be used later to configure our Dex connector.

## 3. Grant Directory Permissions

Our App Registration will need some permissions to be utilized in authentication.

1. Navigate to your new app registration
2. Navigate to the app registration's **API Permissions**
3. Click **Add a permission**
4. Click **Microsoft.Graph**
5. Click **Delegated permissions**
6. Search `Directory.Read.All`
7. Ensure `Directory.Read.All` is enabled
8. Click **Add permissions**
9. Using the information from the previous steps, following the substituted link:
	- `https://login.microsoftonline.com/<tenant-id>/adminconsent?client_id=<client-id>`

We should now be able to plug these details into Dex and configure SSO.

## 4. Add Connector to Dex Configuration

Dex provides [documentation](https://dexidp.io/docs/connectors/microsoft/) on the exact process you may follow to set up SSO through Microsoft Entra ID.  Once you have created your **App Registration** and its respective **Client Secret**, we are ready to configure Dex with the resulting information:

- **Tenant ID**
- **Client ID** (for the App Registration, from Step #1)
- **Client Secret** (from Step #2)
- **Redirect URI**

We need to update the `dex-config` secret within the `epinio` namespace (depending on your installation namespace).  We will append the following block to the bottom of our config at secret key `config.yaml`:

```
connectors:
  - type: microsoft
    # Required field for connector id.
    id: microsoft
    # Required field for connector name.
    name: Microsoft
    config:
      clientID: <client-id>
      clientSecret: <client-secret>
      redirectURI: https://auth.<your-domain>/callback
      tenant: <tenant-id>
```

Restart or redeploy the **Dex** deployment in your cluster.  The new connector 