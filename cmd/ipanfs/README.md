# ipanfs - Cache Panasas user/group quotas

ipanfs is a simple program to cache user/group quotas from Panasas and allow
users to query them using the [iquota](https://github.com/ubccr/iquota) client.
This program is meant to be run via cron and will connect to panfs via ssh and
run the `userquota usage` command and cache the results in redis.

## Install

First create an environment file `/etc/iquota/ipanfs.env`:

```
# Path to where you have panasas mounted
IPANFS_PREFIX=/panasas
# IP and Port of panasas server
IPANFS_ADDRESS=10.1.1.1:22
# SSH Username 
IPANFS_USER=guest
# SSH Password 
IPANFS_PASSWORD=xxx
# Path to ssh key 
IPANFS_KEY=/path/to/ssh/key
# Time in seconds to expire records in redis
IPANFS_EXPIRE=500
```

Then setup to run in cron:

```
0 5 * * * . /etc/iquota/ipanfs.env; /usr/local/bin/ipanfs
```
