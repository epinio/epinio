# Building an epinio rpm for local system install

This document describes building a local x86_64 (amd64) rpm for system install from the provided rpm spec file (epinio.spec).

This process also assumes that a rpm build environment is installed on the system, eg go, rpmbuild etc.

## Prerequisites

- Prerequisite packages to be installed prior to building;
    - golang-packaging.
    - helm.
    - statik (this is in the openSUSE Build Service devel:languages:go repository).
    https://build.opensuse.org/package/show/devel:languages:go/golang-github-rakyll-statik

## Building and installing the epinio rpm

- The build process is carried out as your __user__, switching to __root user__ is only required to install the completed rpm.

```
cd
mkdir -p ~/rpmbuild/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
cp Downloads/epinio.spec rpmbuild/SPECS/
cp Downloads/epinio-0.0.18.tar.gz rpmbuild/SOURCES/
cd rpmbuild/SPECS/
:~/rpmbuild/SPECS> rpmbuild -ba epinio.spec

...build output...

Wrote: /home/username/rpmbuild/SRPMS/epinio-0.0.18-0.src.rpm
Wrote: /home/username/rpmbuild/RPMS/x86_64/epinio-0.0.18-0.x86_64.rpm
Executing(%clean): /bin/sh -e /var/tmp/rpm-tmp.iXjf4j
+ umask 022
+ cd /home/username/rpmbuild/BUILD
+ cd epinio-0.0.18
+ /usr/bin/rm -rf /home/username/rpmbuild/BUILDROOT/epinio-0.0.18-0.x86_64
+ RPM_EC=0
++ jobs -p
+ exit 0

:~/rpmbuild/SPECS> su -
Password:

:~ # zypper in /home/username/rpmbuild/RPMS/x86_64/epinio-0.0.18-0.x86_64.rpm

The following NEW package is going to be installed:
  epinio

1 new package to install.
Overall download size: 9.3 MiB. Already cached: 0 B. After the operation, additional 40.3 MiB will be used.
Continue? [y/n/v/...? shows all options] (y): y
Retrieving package epinio-0.0.18-0.x86_64                                                                                                (1/1),   9.3 MiB ( 40.3 MiB unpacked)
epinio-0.0.18-0.x86_64.rpm:
    Package is not signed!

epinio-0.0.18-0.x86_64 (Plain RPM files cache): Signature verification failed [6-File is unsigned]
Abort, retry, ignore? [a/r/i] (a): i

Checking for file conflicts: ..............................................[done]
(1/1) Installing: epinio-0.0.18-0.x86_64 ..................................[done]

:~ # exit

:~/rpmbuild/SPECS> epinio version

Epinio Version: v0.0.18
Go Version: go1.16.5
```
__NOTE:__ The above process has been tested __only on openSUSE Tumbleweed__.

