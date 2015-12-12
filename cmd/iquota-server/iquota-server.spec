%define __spec_install_post %{nil}
%define debug_package %{nil}

Summary:       Proxy server for Isilon OneFS SmartQuota reporting
Name:          iquota-server
Version:       0.0.1
Release:       1%{?dist}
License:       BSD
Group:         Applications/Internet
SOURCE:        %{name}-%{version}-linux-amd64.tar.gz
URL:           https://github.com/ubccr/iquota
BuildRoot:     %{_tmppath}/%{name}-%{version}-%{release}-root
Requires(pre): /usr/sbin/useradd, /usr/bin/getent

%description
Linux CLI tools for Isilon OneFS SmartQuota reporting

%pre
getent group iquota &> /dev/null || \
groupadd -r iquota &> /dev/null
getent passwd iquota &> /dev/null || \
useradd -r -g iquota -d %{_datadir}/%{name} -s /sbin/nologin \
        -c 'iquota Server' iquota &> /dev/null

%prep
%setup -q -n %{name}-%{version}-linux-amd64

%build
# TODO: consider actually building from source with "go build"

%install
rm -rf %{buildroot}
install -d %{buildroot}%{_datadir}/%{name}
install -d %{buildroot}%{_sysconfdir}/iquota
install -d %{buildroot}%{_bindir}
install -d %{buildroot}%{_usr}/lib/systemd/system

cp -a ./iquota.yaml.sample %{buildroot}%{_sysconfdir}/iquota/iquota.yaml
cp -a ./%{name} %{buildroot}%{_bindir}/%{name}
cat << EOF > %{buildroot}%{_usr}/lib/systemd/system/%{name}.service
[Unit]
Description=iquota server
After=syslog.target
After=network.target

[Service]
Type=simple
User=iquota
Group=iquota
WorkingDirectory=%{_datadir}/%{name}
ExecStart=/bin/bash -c '%{_bindir}/%{name}'
Restart=on-abort

[Install]
WantedBy=multi-user.target
EOF

%clean
rm -rf %{buildroot}

%files
%defattr(-,root,root,-)
%doc README.rst AUTHORS.rst ChangeLog.rst iquota.yaml.sample
%license LICENSE
%attr(0755,root,root) %{_bindir}/%{name}
%attr(640,root,iquota) %config(noreplace) %{_sysconfdir}/iquota/iquota.yaml
%attr(644,root,root) %{_usr}/lib/systemd/system/%{name}.service

%changelog
* Fri Dec 12 2015  Andrew E. Bruno <aebruno2@buffalo.edu> 0.0.1-1
- Initial release
