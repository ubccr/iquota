%define __spec_install_post %{nil}
%define debug_package %{nil}

Summary:       Isilon OneFS SmartQuota report CLI tool
Name:          iquota
Version:       0.0.1
Release:       1%{?dist}
License:       BSD
Group:         Applications/Internet
SOURCE:        %{name}-%{version}-linux-amd64.tar.gz
URL:           https://github.com/ubccr/iquota
BuildRoot:     %{_tmppath}/%{name}-%{version}-%{release}-root

%description
Linux CLI tools for Isilon OneFS SmartQuota reporting

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
%attr(640,root,iquota) %config(noreplace) %{_sysconfdir}/%{name}/%{name}.yaml

%changelog
* Fri Dec 12 2015  Andrew E. Bruno <aebruno2@buffalo.edu> 0.0.1-1
- Initial release
