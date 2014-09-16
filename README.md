# Neverdown

Distributed website monitoring system that triggers WebHooks when a website status change.

## Features

- A simple HTTP JSON API, no UI.
- Distributed using [raft](https://github.com/hashicorp/raft) (a 3 nodes cluster can tolerate one failure).
- Trigger WebHooks when a website status change (down->up/up->down), if a WebHook is not received, it will be retried up to 20 times (with exponential backoff).

## API endpoints

You can query any server, query will automatically be redirected to the leader, just ensure you are following redirections (e.g. the ``-L`` flag using ``curl``.

```console
$ curl -L http://localhost:7990/check
{"checks":[]}
```

Endpoints with the **_** prefix, like _ping, are special node endpoints and are not redirected to the leader.

### GET /check

List all checks.

```console
$ curl http://localhost:7990/check
{
    "checks": [
	{
            "first_check": 1408978031, 
            "id": "trucsdedev", 
            "interval": 60, 
            "last_check": 1408978091, 
            "last_down": 1408978037, 
            "last_error": {
                "error": "no such host", 
                "status_code": 0, 
                "type": "dns"
            }, 
            "method": "HEAD", 
            "outages": 1, 
            "pings": 2, 
            "time_down": 120, 
            "time_up": 0, 
            "up": false, 
            "uptime": 0, 
            "url": "http://trucsdedev.com", 
            "webhooks": []
        },
    ]
}
```

### POST /check

Create/update a check (if you POST a check with an existing id, it will replace the existing check).

You can specify an custom **id**, if no id is specified, a random UUID will be generated.

**HEAD** is the default **method**, but you can specify any HTTP method.

The default **interval** is 60 seconds.

```console
$ curl -XPOST http://localhost:7990/check -d '{"id": "trucsdedev", "interval": 60, "url": "http://trucsdedev.com", "webhooks":["http://requestb.in/18myl7y1"]}'
```

### GET /check/{id}

Retrieve a single check by id.

```console
$ curl http://localhost:7990/check/trucsdedev
{
    "first_check": 1408978031, 
    "id": "trucsdedev", 
    "interval": 60, 
    "last_check": 1408978091, 
    "last_down": 1408978037, 
    "last_error": {
	"error": "no such host", 
	"status_code": 0, 
	"type": "dns"
    }, 
    "method": "HEAD", 
    "outages": 1, 
    "pings": 2, 
    "time_down": 120, 
    "time_up": 0, 
    "up": false, 
    "uptime": 0, 
    "url": "http://trucsdedev.com", 
    "webhooks": []
}
```

### DELETE /check/{id}

Delete a check.

```console
$ curl -XDELETE http://localhost:7990/check/trucsdedev
```

### GET /pending

List all pending webhooks.

```console
$ curl http://localhost:7990/pending
{
    "pending": [
        {
            "id": "c2cc7440-75b8-4e61-9608-b68f39c58013",
            "url": "http://trucsdedev.com",
            "payload": "eyJpZCI6Im[...]Y3NkZWRldi5jb20iXX0=",
            "tries": 5,
            "first_try": 1407262636
        }
    ]
}
```


### GET /pending/{id}

Retrieve a pending webhook.

```console
$ curl http://localhost:7990/pending/c2cc7440-75b8-4e61-9608-b68f39c58013
{
    "id": "c2cc7440-75b8-4e61-9608-b68f39c58013",
    "url": "http://trucsdedev.com",
    "payload": "eyJpZCI6ImJsb[...]Rldi5jb20iXX0=",
    "tries": 7,
    "first_try": 1407262636
}
```

### DELETE /pending/{id}

Force the delete of a pending WebHook.

```console
$ curl -XDELETE http://localhost:7990/pending/c2cc7440-75b8-4e61-9608-b68f39c58013
```

### GET /_ping

Special endpoints used by the leader to query followers.

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

Fetch cluster infos.

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

- **timeout**: the 10 seconds timeout has been exceeded while loading the page.
- **dns**: there is a DNS issue.
- **server**: server issue, refers to the status code and the error returned by the server.
- **unknown**: unknown or not handled yet issue.

## Docker

A [Dockerfile](.docker/Dockerfile) is available, you can easily build the Docker image:

```console
$ make docker
```

You can also get it on via the [Docker Hub](https://registry.hub.docker.com/u/neverdown/neverdown/).

### Deploying with Docker

```console
$ sudo docker pull neverdown/neverdown
$ sudo docker run -p 8001:8000 -p 7990:7990 -e NEVERDOWN_PEERS=:8000,:8001 -t neverdown/neverdown
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
$ NEVERDOWN_ADDR=:8000 NEVERDOWN_PREFIX=ok NEVERDOWN_PEERS=:8000,:8001,:8002 ./neverdown
```

## TODO

- Handle more error type and provides more user-friendly error message

## License

Copyright (c) 2014 Thomas Sileo and contributors. Released under the MIT license.
