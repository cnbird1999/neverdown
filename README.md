# Neverdown

Distributed website monitoring system that triggers WebHooks when a website status change.

## Features

- A simple HTTP JSON API (using [Hawk](https://github.com/hueniverse/hawk) authentication)
- Distributed using [raft](https://github.com/hashicorp/raft) (a 3 nodes cluster can tolerate one failure)
- Trigger WebHooks when a website status change (down->up/up->down)

## API endpoints

The API uses Hawk as authentication mechanism, to play with the API from the command-line,
the easiest way is to install [HTTPie](http://httpie.org) (with [requests-auth](https://github.com/mozilla-services/requests-hawk)).

```console
$ sudo pip install --upgrade httpie requests-hawk
$ http GET 192.168.1.20:7990/_cluster --auth-type=hawk --auth='thomas:debug'
HTTP/1.1 200 OK
Content-Length: 29
Content-Type: application/json
Date: Sun, 03 Aug 2014 18:59:01 GMT

{
    "leader": ":7990", 
    "peers": []
}
```

### GET /check

### POST /check

### GET /check/{id}

### DELETE /check/{id}

### GET /_ping

### GET /_cluster

```console
$ http GET 192.168.1.20:7990/_cluster --auth-type=hawk --auth='thomas:debug'
HTTP/1.1 200 OK
Content-Length: 29
Content-Type: application/json
Date: Sun, 03 Aug 2014 18:59:01 GMT

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

## TODO

- Leader redirection for the HTTP API

## License

Copyright (c) 2014 Thomas Sileo and contributors. Released under the MIT license.
