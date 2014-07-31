# Monitoring

Distributed website monitoring system that triggers webhooks when a website is down.

## Features

- A simple HTTP JSON API
- Distributed using [raft](https://github.com/hashicorp/raft) (a 3 nodes cluster can tolerate one failure)
- Trigger WebHooks when a webiste status change (down->up/up->down)

## API

### GET /check

### POST /check

### GET /check/{id}

### DELETE /check/{id}

## Limitations



## License

Copyright (c) 2014 Thomas Sileo and contributors. Released under the MIT license.
