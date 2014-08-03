# Neverdown

Distributed website monitoring system that triggers WebHooks when a website status change.

## Features

- A simple HTTP JSON API
- Distributed using [raft](https://github.com/hashicorp/raft) (a 3 nodes cluster can tolerate one failure)
- Trigger WebHooks when a website status change (down->up/up->down)

## API endpoints

### GET /check

### POST /check

### GET /check/{id}

### DELETE /check/{id}

### GET /_ping

### GET /_cluster

```console
$ curl http://localhost:7990/_cluster
{
    "leader": ":7990", 
    "peers": []
}
```

## Deploying with Docker

A [Dockerfile](.docker/Dockerfile) is available.

**Image not pushed on the docker hub yet.**

```console
$ sudo docker pull tsileo/neverdown
$ sudo docker run -p 8000:8000 -p 7000:7000 -v /tmp/neverdown_data/:/data/neverdown -e UPCHECK_PEERS=host1:8000,host2:8000;host3:8000 -t tsileo/neverdown
```
## Security

### Raft

You should setup ssh tunnels and listen only on local interfaces.

```console
$ autossh -f -NL 8001:127.0.0.1:8001 user@remote_host
```

## TODO

- Leader redirection for the HTTP API

## License

Copyright (c) 2014 Thomas Sileo and contributors. Released under the MIT license.
