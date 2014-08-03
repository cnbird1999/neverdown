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


## TODO

- Leader redirection for the HTTP API
- Implements WebHooks

## License

Copyright (c) 2014 Thomas Sileo and contributors. Released under the MIT license.
