#!/bin/bash

set -e

revision="${1}"
if test "$revision" == ""
then
    if test "${REV}" != ""
    then
	revision="${REV}"
    else
	echo 1>&2 'Usage:' $0 revision
	echo 1>&2 'Or:   ' REV=revision $0
	exit 1
    fi
fi

echo Updating application crd to ${revision} ...

go get -d "github.com/epinio/application@${revision}"
(
    echo    '# Copied from here:'
    echo -n '# https://github.com/epinio/application/blob/main/config/crd/bases/application.epinio.io_apps.yaml'
    cat ../application/config/crd/bases/application.epinio.io_apps.yaml
) > assets/embedded-files/epinio/app-crd.yaml

echo /Done
exit
