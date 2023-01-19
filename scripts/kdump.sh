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

dst="${1:-resources}"

rm -rf   "${dst}"
mkdir -p "${dst}"

kubectl api-resources >> "${dst}/list-all"

grep -v NAME  "${dst}/list-all" | while read name short api remainder
do
    case $short in
	*v1*)
	    # No shortname, api version is here. Shuffle around.
	    api="${short}"
	    short=""
	    ;;
	*) ;;
    esac

    api="${api%/*}"
    if test "$api" = "v1" ; then
	api=""
    else
	api=".${api}"
    fi

    r="${name}${api}"

    echo _ _ __ ___ _____ ________ _____________ _____________________ $r

    kubectl get $r --all-namespaces > $$.data 2> $$.err
    if test -s $$.err
    then
	echo >> "${dst}/list-err" $r
	( echo _ _ __ ___ _____ ________ _____________ _____________________ $r
	  cat $$.err
	) >> "${dst}/errors"
	continue
    else
	echo >> "${dst}/list-ok" $r
	( echo _ _ __ ___ _____ ________ _____________ _____________________ $r
	  cat $$.data
	) >> "${dst}/ok"
    fi
    entries="$(grep -v NAME $$.data | wc -l)"
    if test $entries -eq 1
    then
	echo "$entries element"
    else
	echo "$entries elements"
    fi

    dr="${dst}/${r}"    
    if test -f "${dr}"
    then
	k=1
	while test -f "${dr}.$k"
	do
	    k = $(( k + 1 ))
	done
	dr="${dr}.$k"
    fi
    mkdir -p "${dr}.d"
    cp -l $$.data "${dr}"

    if test "$r" = "events"               ; then rmdir "${dr}.d" ; rm $$.* ; continue ; fi
    if test "$r" = "events.events.k8s.io" ; then rmdir "${dr}.d" ; rm $$.* ; continue ; fi

    if grep NAMESPACE $$.data >/dev/null 2>&1 && grep NAME $$.data >/dev/null 2>&1
    then
	# Namespaced resource
	grep -v NAME $$.data | while read namespace name args
	do
	    echo -e "\t$namespace/$name"
	    kubectl get $r -n "${namespace}" $name -o yaml > "${dr}.d/${namespace}---${name}"
	done
    else
	# Cluster global resource
	grep -v NAME $$.data | while read name args
	do
	    echo -e "\t$name"
	    kubectl get $r $name -o yaml > "${dr}.d/${name}"
	done
    fi
    rm $$.*
done
