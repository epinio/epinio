#!/bin/bash

#set -x

DEX_URL=https://auth.172.21.0.4.omg.howdoi.website
CLIENT_ID=epinio-cli
USERNAME=admin@epinio.io
PASSWORD=password

STATE=$(echo $RANDOM | md5sum | cut -d' ' -f1)
# PKCE challenge setup
CODE_VERIFIER=$(echo $RANDOM | md5sum | cut -d' ' -f1)
CODE_CHALLENGE=$(echo -n $CODE_VERIFIER | sha256sum | cut -d' ' -f1 | xxd -r -p | base64 -w0 | sed 's#+#-#' | sed 's#/#_#' | sed 's#=##')

echo $STATE
echo $CODE_VERIFIER
echo $CODE_CHALLENGE

LOGIN_URL=$(
    curl -vskL --cookie cookie.txt --cookie-jar cookie.txt \
        -d "redirect_uri=urn:ietf:wg:oauth:2.0:oob" \
        -d "client_id=$CLIENT_ID" \
        -d "scope=openid+offline_access+profile+email+groups" \
        -d "state=$STATE" \
        -d "code_challenge_method=S256" \
        -d "response_type=code" \
        -d "code_verifier=$CODE_VERIFIER" \
        -d "code_challenge=$CODE_CHALLENGE" \
        $DEX_URL/auth/local 2>&1 \
    | grep '< location: /auth/local/login' \
    | cut -d ' ' -f3 \
    | tr -dc '[[:print:]]'
)

echo $DEX_URL$LOGIN_URL

APPROVE_URL=$(
    curl -vkL --cookie cookie.txt --cookie-jar cookie.txt \
        -d "login=$USERNAME" \
        -d "password=$PASSWORD" \
        $DEX_URL$LOGIN_URL 2>&1 \
    | grep '< location: /approval' \
    | cut -d ' ' -f3 \
    | tr -dc '[[:print:]]'
)

echo $DEX_URL$APPROVE_URL

AUTH_CODE=$(
    curl -skL --cookie cookie.txt --cookie-jar cookie.txt \
        -d "approval=approve" \
        $DEX_URL$APPROVE_URL \
    | grep 'value' \
    | cut -d '"' -f6 \
    | tr -dc '[[:print:]]'
)

echo $AUTH_CODE

ACCESS_TOKEN=$(
    curl -ks \
        -d "redirect_uri=urn:ietf:wg:oauth:2.0:oob" \
        -d "grant_type=authorization_code" \
        -d "client_id=$CLIENT_ID" \
        -d "code_verifier=$CODE_VERIFIER" \
        -d "code=$AUTH_CODE" \
        ${DEX_URL}/token
)

echo $ACCESS_TOKEN