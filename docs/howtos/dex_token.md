# Dex token

In the `docs` folder there is a `login.sh` script that can be used to perform a PKCE flow with the Dex instance, and get a valid token.

To use it just execute it with `username`, `password`, and `dex_url`:

```sh
./docs/login.sh 'admin@epinio.io' password https://auth.172.21.0.4.sslip.io
```

Running it with the `-v` flag will print some information and will show the decoded claims:

```sh
$ ./docs/login.sh 'admin@epinio.io' password https://auth.172.21.0.4.sslip.io

[INFO] Getting auth URL for user 'admin@epinio.io' to 'https://auth.172.21.0.4.sslip.io'
[INFO]  - state: '8f296829f25a13fb2cd52a04e0d70937'
[INFO]  - code_verifier: '23ff15b78acf3fb889ef54ef03dac341'
[INFO]  - code_challenge: 'XULPaEsw7BTNwN0rURgBj4hRli8i9UYBumi844q0FgA'
[INFO] Auth URL: https://auth.172.21.0.4.sslip.io/auth/local/login?back=&state=xodeyy75w4ce6bne6wdtt37ax
[INFO] Approve URL: https://auth.172.21.0.4.sslip.io/approval?req=xodeyy75w4ce6bne6wdtt37ax
[INFO] Got Authorization Code: 'uhhiv472bgvehsllptcgi64cf'

[INFO] Got Token
{
  "access_token": "eyJhbGciOiJSUzI1NiIsImtpZCI6ImNjYmVlZTFlMWIzNWYzZjVmOGI0OGJhYjRlNDg4OWEzYWQzZjJlMGIifQ.eyJpc3MiOiJodHRwczovL2F1dGguMTcyLjIxLjAuNC5vbWcuaG93ZG9pLndlYnNpdGUiLCJzdWIiOiJDaVF3T0dFNE5qZzBZaTFrWWpnNExUUmlOek10T1RCaE9TMHpZMlF4TmpZeFpqVTBOallTQld4dlkyRnMiLCJhdWQiOiJlcGluaW8tY2xpIiwiZXhwIjoxNjY0MzYwMTg4LCJpYXQiOjE2NjQyNzM3ODgsImF0X2hhc2giOiJXTnc5YWladExtdzd4QzJobTA1NWxBIiwiZW1haWwiOiJhZG1pbkBlcGluaW8uaW8iLCJlbWFpbF92ZXJpZmllZCI6dHJ1ZSwibmFtZSI6ImFkbWluIn0.N05mNRH9xBJTU0JNNSqxsA-YFWT4rOa2O4Tf75kiYZxttQYfjgs__7PyTzSxG5XP4-yk7Zhig9x_yW48aBMKkvKjogL3peVP-idAQMK5Ultv8kEjBAOU9EOgKMvlda49DP1zmxSvjTfLdiJQPW3yEtuLAwvsycUqcyrstKfaCMQKP9LWkwFaAypYuJHvZOvwYczI3E-caoMbGeYhj1qnnqytPDj3zz5kr3zwxFbO6wJHcO0k56wO-hQ8jf490kRp-rlOSiKgNvb9D_bu7k259v9C_AW-crZS2qcVXdaUz_Mj74wTX8FzFiQaJygIrZ9E5ZJcuiDg5ysOsuBjA1v41g",
  "token_type": "bearer",
  "expires_in": 86399,
  "refresh_token": "ChlxamY3bzdlZHFlY3EyZ3pjbTNjNHprdnh1Ehluc2FyaXlwNGlnazJiNGgzYXc1cHV4NzNl",
  "id_token": "eyJhbGciOiJSUzI1NiIsImtpZCI6ImNjYmVlZTFlMWIzNWYzZjVmOGI0OGJhYjRlNDg4OWEzYWQzZjJlMGIifQ.eyJpc3MiOiJodHRwczovL2F1dGguMTcyLjIxLjAuNC5vbWcuaG93ZG9pLndlYnNpdGUiLCJzdWIiOiJDaVF3T0dFNE5qZzBZaTFrWWpnNExUUmlOek10T1RCaE9TMHpZMlF4TmpZeFpqVTBOallTQld4dlkyRnMiLCJhdWQiOiJlcGluaW8tY2xpIiwiZXhwIjoxNjY0MzYwMTg4LCJpYXQiOjE2NjQyNzM3ODgsImF0X2hhc2giOiJJZ2VDVGdoTVd1cUNMSlUtN0xMM1NRIiwiY19oYXNoIjoiRThzSkRiT0d3Q09WSVBnTWFPZEFYQSIsImVtYWlsIjoiYWRtaW5AZXBpbmlvLmlvIiwiZW1haWxfdmVyaWZpZWQiOnRydWUsIm5hbWUiOiJhZG1pbiJ9.ftWMVavGL2nXhkBortDNvyFDVeZ31OtE-xIGHf6O-s5kfxxO_4BfSNECgmWFHyylnRYjH78Hb1dALR7KB9Nki5QvYE_aUDfnCPwLJNfGtlKxswhAGkPWRPatBsnKjUu7lH2bbzO69nJWHgU_QMJt7foQw7_pYOf9El2KRljxGgxlN5lu37P7lsm-F7aYu61exqK-jAqbkJN_G46XiLryKqXdiHIGpWMl5sIUeh-R7brvToSavEmCD4bjOIm65u6VX6-S-WzcG-1vUH4IvTxCX4yIgiRL8G-OkkbfbyJSm4-s1UGlPIZS01Df8-WaXgGlKD8JylFodC5cYapL0P6ocg"
}

[INFO] Decoded claims
{
  "iss": "https://auth.172.21.0.4.sslip.io",
  "sub": "CiQwOGE4Njg0Yi1kYjg4LTRiNzMtOTBhOS0zY2QxNjYxZjU0NjYSBWxvY2Fs",
  "aud": "epinio-cli",
  "exp": 1664360188,
  "iat": 1664273788,
  "at_hash": "WNw9aiZtLmw7xC2hm055lA",
  "email": "admin@epinio.io",
  "email_verified": true,
  "name": "admin"
}
eyJhbGciOiJSUzI1NiIsImtpZCI6ImNjYmVlZTFlMWIzNWYzZjVmOGI0OGJhYjRlNDg4OWEzYWQzZjJlMGIifQ.eyJpc3MiOiJodHRwczovL2F1dGguMTcyLjIxLjAuNC5vbWcuaG93ZG9pLndlYnNpdGUiLCJzdWIiOiJDaVF3T0dFNE5qZzBZaTFrWWpnNExUUmlOek10T1RCaE9TMHpZMlF4TmpZeFpqVTBOallTQld4dlkyRnMiLCJhdWQiOiJlcGluaW8tY2xpIiwiZXhwIjoxNjY0MzYwMTg4LCJpYXQiOjE2NjQyNzM3ODgsImF0X2hhc2giOiJXTnc5YWladExtdzd4QzJobTA1NWxBIiwiZW1haWwiOiJhZG1pbkBlcGluaW8uaW8iLCJlbWFpbF92ZXJpZmllZCI6dHJ1ZSwibmFtZSI6ImFkbWluIn0.N05mNRH9xBJTU0JNNSqxsA-YFWT4rOa2O4Tf75kiYZxttQYfjgs__7PyTzSxG5XP4-yk7Zhig9x_yW48aBMKkvKjogL3peVP-idAQMK5Ultv8kEjBAOU9EOgKMvlda49DP1zmxSvjTfLdiJQPW3yEtuLAwvsycUqcyrstKfaCMQKP9LWkwFaAypYuJHvZOvwYczI3E-caoMbGeYhj1qnnqytPDj3zz5kr3zwxFbO6wJHcO0k56wO-hQ8jf490kRp-rlOSiKgNvb9D_bu7k259v9C_AW-crZS2qcVXdaUz_Mj74wTX8FzFiQaJygIrZ9E5ZJcuiDg5ysOsuBjA1v41g
```


