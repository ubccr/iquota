===============================================================================
iquota - Linux CLI tools for Isilon OneFS SmartQuota reporting
===============================================================================

------------------------------------------------------------------------
What is iquota?
------------------------------------------------------------------------

- TODO

------------------------------------------------------------------------
Features
------------------------------------------------------------------------

- User/Group quota reporting from command line
- Kerberos based authentication
- Caching via redis

------------------------------------------------------------------------
Requirements
------------------------------------------------------------------------

- Isilon OneFS API (v7.2.1)
- Linux
- Kerberos

------------------------------------------------------------------------
Install
------------------------------------------------------------------------

- TODO

------------------------------------------------------------------------
Configure caching
------------------------------------------------------------------------

iquota-server can optionally be configured to cache results for a given time
period. To enable caching first install redis then update
/etc/iquota/iquota.yaml.

Install Redis (install from EPEL)::

    $ yum install https://dl.fedoraproject.org/pub/epel/epel-release-latest-7.noarch.rpm
    $ yum install redis
    $ systemctl restart redis
    $ systecmtl enable redis

Edit /etc/iquota/iquota.yaml and restart::

    $ vi /etc/iquota/iquota.yaml
    enable_caching: true

    $ systecmtl restart iquota-server

------------------------------------------------------------------------
License
------------------------------------------------------------------------

iquota is released under a BSD style license. See the LICENSE file. 
