# Neverdown

Distributed website monitoring system that triggers WebHooks when a website status change.

## Features

- A simple HTTP JSON API, no UI.
- Distributed using [raft](https://github.com/hashicorp/raft) (a 3 nodes cluster can tolerate one failure).
- Trigger WebHooks when a website status change (down->up/up->down), if a WebHook is not received, it will be retried up to 20 (with exponential backoff)

## API endpoints

### GET /check

```console
$ curl http://localhost:7990/check
{
    "checks": [
        {
            "id": "trucsdedev",
            "url": "http://trucsdedev.com",
            "method": "HEAD",
            "last_check": 1407236438,
            "last_error": {
                "error": "no such host",
                "status_code": 0,
                "type": "dns"
            },
            "up": false,
            "last_down": 1407236378,
            "interval": 60,
            "webhooks": [
                "http://trucsdedev.com"
            ]
        }
    ]
}
```

### POST /check

You can specify an custom id, if no id is specified, a random UUID will be generated.

```console
$ curl -XPOST http://localhost:7990/check -d '{"id": "trucsdedev", "interval": 60, "url": "http://trucsdedev.com", "webhooks":["http://requestb.in/18myl7y1"]}'
```

### GET /check/{id}

```console
$ curl http://localhost:7990/check/trucsdedev
{
    "id": "trucsdedev",
    "url": "http://trucsdedev.com",
    "method": "HEAD",
    "last_check": 1407236438,
    "last_error": {
        "error": "no such host",
        "status_code": 0,
        "type": "dns"
    },
    "up": false,
    "last_down": 1407236378,
    "interval": 60,
    "webhooks": [
        "http://trucsdedev.com"
    ]
}
```

### DELETE /check/{id}

```console
$ curl -XDELETE http://localhost:7990/check/trucsdedev
```

### GET /_ping

```console
$ curl http://localhost:7990/_ping\?url\=http://trucsdedev.com
{
    "url": "http://trucsdedev.com",
    "up": false,
    "error": {
        "status_code": 0,
        "type": "dns",
        "error": "no such host"
    }
}
$ curl http://localhost:7990/_ping\?url\=http://google.com
{
    "url": "http://google.com",
    "up": true,
    "error": {
        "status_code": 200,
        "type": "",
        "error": ""
    }
}
```

### GET /_cluster

```console
$ curl http://localhost:7990/_cluster
{
    "leader": ":7990", 
    "peers": []
}
```

## WebHooks

When a website status change, the provided webhooks will be executed,
if a webhook is not received, it will be retried up to 20 times (with exponential backoff).

## Payload

```json
{
    "id": "trucsdedev",
    "url": "http://trucsdedev.com",
    "method": "HEAD",
    "last_check": 1407240244,
    "last_error": {
        "status_code": 0,
        "type": "dns",
        "error": "no such host"
    },
    "up": false,
    "last_down": 1407240185,
    "interval": 60,
    "webhooks": [
        "http://requestb.in/tqxmuntq"
    ]
}
```

### Error type

- **timeout**
- **dns**
- **server**
- **unknow**

## Deploying with Docker

A [Dockerfile](.docker/Dockerfile) is available.

**Image not pushed on the docker hub yet.**

```console
$ sudo docker pull neverdown/neverdown
$ sudo docker run -p 8000:8000 -p 7000:7000 -v /tmp/neverdown_data/:/data/neverdown -e UPCHECK_PEERS=host1:8000,host2:8000;host3:8000 -t neverdown/neverdown
```
## Security

### Raft

You should setup ssh tunnels and listen only on local interfaces.

```console
$ autossh -f -NL 8001:127.0.0.1:8001 user@remote_host
```

## Running locally

Configuration is handled by environment variable.

```console
$ UPCHECK_ADDR=:8000 UPCHECK_PREFIX=ok UPCHECK_PEERS=:8000,:8001,:8002 ./neverdown
```

## TODO

- Leader redirection for the HTTP API

## License

Copyright (c) 2014 Thomas Sileo and contributors. Released under the MIT license.
