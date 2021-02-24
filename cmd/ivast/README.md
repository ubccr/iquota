# ivast - Cache VAST quotas

ivast is a simple program to cache volume quotas from VAST storage system and
allow users to query them using the [iquota](https://github.com/ubccr/iquota)
client.  This program is meant to be run via cron and will connect to the VAST
API.  . Results are cached in redis.

## Install

First create an environment file `/etc/iquota/ivast.env`:

```
# Hostname of VAST API server
VAST_HOST=10.1.1.1
# Username 
VAST_USER=guest
# Password 
VAST_PASSWORD=xxx
# Time in seconds to expire records in redis
VAST_EXPIRE=500
```

Then setup to run in cron:

```
0 5 * * * . /etc/iquota/ivast.env; /usr/local/bin/ivast
```
