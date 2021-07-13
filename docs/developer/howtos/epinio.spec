#
# spec file for package epinio
#
# Copyright (c) 2021 SUSE LLC
#
# All modifications and additions to the file contributed by third parties
# remain the property of their copyright owners, unless otherwise agreed
# upon. The license for this file, and modifications and additions to the
# file, is the same license as for the pristine package itself (unless the
# license for the pristine package is not an Open Source License, in which
# case the license is the MIT License). An "Open Source License" is a
# license that conforms to the Open Source Definition (Version 1.9)
# published by the Open Source Initiative.

# Please submit bugfixes or comments via http://bugs.opensuse.org/
#

%global provider        github
%global provider_tld    com
%global project         epinio
%global repo            epinio
%global provider_prefix %{provider}.%{provider_tld}/%{project}/%{repo}
%global import_path     %{provider_prefix}

Name:           epinio
#NOTE: Update version on new version release
Version:        0.0.18
Release:        0
License:        Apache-2.0
Summary:        From App to URL in one step
URL:            https://epinio.io/
Source0:        %{repo}-%{version}.tar.gz
BuildRequires:  golang-packaging
BuildRequires:  helm
BuildRequires:  statik

%{go_nostrip}
%{go_provides}

%description
Epinio is an opinionated platform that runs on Kubernetes and takes you from App
to URL in one step.

%prep
%autosetup -n %{repo}-%{version}

%build
%goprep %{import_path}
# Need to embed assests
helm package ./assets/container-registry/chart/container-registry/ -d assets/embedded-files
statik -m -f -src=./assets/embedded-files -dest assets
statik -m -f -src=./assets/embedded-web-files/views -ns webViews -p statikWebViews -dest assets
statik -m -f -src=./assets/embedded-web-files/assets -ns webAssets -p statikWebAssets -dest assets

export GOARCH="amd64"
export GOOS="linux"
export VERSION="v%{version}"
export CGO_ENABLED=0
go build \
   -ldflags "-s -w -X github.com/epinio/epinio/internal/version.Version=$VERSION" \
   -o epinio ;

%install
install -d -m 0755 %{buildroot}%{_bindir}
install -m 0755 epinio %{buildroot}/%{_bindir}/epinio

%files -n epinio
%license LICENSE
%{_bindir}/epinio

%changelog
* Mon Jul 12 17:22:06 UTC 2021 - Malcolm Lewis <malcolmlewis@opensuse.org>
- Initial spec file tested with v0.0.18.


