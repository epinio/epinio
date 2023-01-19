#!/bin/bash
# Copyright © 2021 - 2023 SUSE LLC
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#     http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#set -x

NORMAL=$(tput sgr0)
GREEN=$(tput setaf 2)
RED=$(tput setaf 1)

function log {
    case $1 in
    info)   LOG_PREFIX="${GREEN}[INFO]${NORMAL}" ;;
    err)    LOG_PREFIX="${RED}[ERR]${NORMAL}" ;;
    *)      LOG_PREFIX="${GREEN}[INFO]${NORMAL}"
    esac
    echo "${LOG_PREFIX} $2"
}

CLIENT_ID=epinio-cli

verbose="false"

while getopts 'up:v' flag; do
  case "${flag}" in
    v) verbose="true" ;;
  esac
done
shift "$(($OPTIND -1))"

USERNAME=$1
PASSWORD=$2
DEX_URL=$3

STATE=$(echo $RANDOM | md5sum | cut -d' ' -f1)
# PKCE challenge setup
CODE_VERIFIER=$(echo $RANDOM | md5sum | cut -d' ' -f1)
CODE_CHALLENGE=$(echo -n $CODE_VERIFIER | sha256sum | cut -d' ' -f1 | xxd -r -p | base64 -w0 | sed -e 's#+#-#g' -e 's#/#_#g' -e 's#=##g')

# Check params
[ -z $USERNAME  ] && log err "Missing username" && exit 1
[ -z $PASSWORD  ] && log err "Missing password" && exit 1
[ -z $DEX_URL   ] && log err "Missing authentication URL" && exit 1

if [ $verbose = "true" ]; then
    log info "Getting auth URL for user '$USERNAME' to '$DEX_URL'"
    log info " - state: '$STATE'"
    log info " - code_verifier: '$CODE_VERIFIER'"
    log info " - code_challenge: '$CODE_CHALLENGE'"
fi

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
        $DEX_URL/auth 2>&1 \
    | grep '< location: /auth/local/login' \
    | cut -d ' ' -f3 \
    | tr -dc '[[:print:]]'
)

[ $verbose = "true" ] && log info "Auth URL: $DEX_URL$LOGIN_URL"

APPROVE_URL=$(
    curl -vkL --cookie cookie.txt --cookie-jar cookie.txt \
        -d "login=$USERNAME" \
        -d "password=$PASSWORD" \
        $DEX_URL$LOGIN_URL 2>&1 \
    | grep '< location: /approval' \
    | cut -d ' ' -f3 \
    | tr -dc '[[:print:]]'
)

[ $verbose = "true" ] && log info "Approve URL: $DEX_URL$APPROVE_URL"

AUTH_CODE=$(
    curl -skL --cookie cookie.txt --cookie-jar cookie.txt \
        -d "approval=approve" \
        $DEX_URL$APPROVE_URL \
    | grep 'value' \
    | cut -d '"' -f6 \
    | tr -dc '[[:print:]]'
)

[ $verbose = "true" ] && log info "Got Authorization Code: '$AUTH_CODE'"

ACCESS_TOKEN=$(
    curl -ks \
        -d "redirect_uri=urn:ietf:wg:oauth:2.0:oob" \
        -d "grant_type=authorization_code" \
        -d "client_id=$CLIENT_ID" \
        -d "code_verifier=$CODE_VERIFIER" \
        -d "code=$AUTH_CODE" \
        ${DEX_URL}/token
)


if [ $verbose = "true" ]; then
    echo
    log info "Got Token"
    echo $ACCESS_TOKEN | jq

    echo
    log info "Decoded claims"
    echo $ACCESS_TOKEN | jq -r .access_token | cut -d '.' -f2 | base64 -d 2>/dev/null | jq
fi

rm cookie.txt

echo $ACCESS_TOKEN | jq -r .access_token
