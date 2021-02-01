%define __spec_install_post %{nil}
%define debug_package %{nil}

Summary:       CCR quota report CLI tool
Name:          iquota
Version:       0.0.6
Release:       1%{?dist}
License:       BSD
Group:         Applications/Internet
SOURCE:        %{name}-%{version}-linux-amd64.tar.gz
URL:           https://github.com/ubccr/iquota
BuildRoot:     %{_tmppath}/%{name}-%{version}-%{release}-root

%description
Linux CLI tools for CCR reporting

%pre

%prep
%setup -q -n %{name}-%{version}-linux-amd64

%build
# TODO: consider actually building from source with "go build"

%install
rm -rf %{buildroot}
install -d %{buildroot}%{_datadir}/%{name}
install -d %{buildroot}%{_sysconfdir}/%{name}
install -d %{buildroot}%{_bindir}

cp -a ./%{name}.yaml.sample %{buildroot}%{_sysconfdir}/%{name}/%{name}.yaml
cp -a ./%{name} %{buildroot}%{_bindir}/%{name}

%clean
rm -rf %{buildroot}

%files
%defattr(-,root,root,-)
%doc README.rst AUTHORS.rst ChangeLog.rst iquota.yaml.sample
%license LICENSE
%attr(0755,root,root) %{_bindir}/%{name}
%attr(644,root,root) %config(noreplace) %{_sysconfdir}/%{name}/%{name}.yaml

%changelog
* Sun Jan 31 2021  Andrew E. Bruno <aebruno2@buffalo.edu> 0.0.6-1
- New Features
    - Add support to client for vast mounts
* Thu Jul 23 2020  Andrew E. Bruno <aebruno2@buffalo.edu> 0.0.5-1
- New Features
    - Add support to client for panfs mounts
* Fri Jul 27 2018  Andrew E. Bruno <aebruno2@buffalo.edu> 0.0.4-1
- Bug fix
    - OneFS sessions not working. Use basic auth
* Tue Oct 04 2016  Andrew E. Bruno <aebruno2@buffalo.edu> 0.0.3-1
- New Features
    - Add support for detecting nfsv4 mounts in iquota client
* Mon Jan 18 2016  Andrew E. Bruno <aebruno2@buffalo.edu> 0.0.2-1
- New Features
    - Add option to fetch all quotas that have exceeded one or more thresholds
    - Add option to print default user/group quotas. Do not show by default
* Fri Dec 12 2015  Andrew E. Bruno <aebruno2@buffalo.edu> 0.0.1-1
- Initial release
