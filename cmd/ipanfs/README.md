# ipanfs - Cache Panasas volume quotas

ipanfs is a simple program to cache volume quotas from Panasas and allow
users to query them using the [iquota](https://github.com/ubccr/iquota) client.
This program is meant to be run via cron and will connect to panfs via ssh and
the PASXML API. Results are cached in redis.

## Install

First create an environment file `/etc/iquota/ipanfs.env`:

```
# Path to where you have panasas mounted
IPANFS_PREFIX=/panasas
# IP of panasas server
IPANFS_ADDRESS=10.1.1.1
# Panfs Username 
IPANFS_USER=guest
# Panfs Password 
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
