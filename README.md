# DOcker NEtwork MANager - or DONEMAN

[Docker Image](https://hub.docker.com/r/ewancoder/doneman)

This tool monitors your docker containers, and makes sure that all of them have required networks connected to it.

## The Why

I'm hosting a bunch of **standalone** docker containers connected to an **overlay** network so that they are accessible by the other containers hosted on another machine. However, my machine is NOT a Swarm Manager - it is a Worker.

When I restart my PC, all of the containers fail to start because Docker doesn't find overlay networks (until the node connects to Swarm Manager), essentially creating an effect of all (or half) of my containers being down after any reboot.

Furthermore, whenever I am redeploying some services - there is a CHANCE that they might not start, because even when node is connected to the Swarm Manager - the overlay network might still NOT be available for some reason to the **standalone** containers, so you might need to restart them multiple times until they successfully start.

## How it works

Doneman monitors the list of services with their required networks and for every service it checks the following:

1. The container is running
2. The container has all required networks connected to it

If any of these checks fail, Doneman will:

1. Stop the container (if it's running)
2. Disconnect it from all required networks
3. Start the container
4. Try to reconnect it to all required networks until successful

## Usage

Just include Doneman in your docker compose file like this:

```yml
services:
  doneman:
    image: ewancoder/doneman:latest
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    restart: unless-stopped
    environment:
      - NETWORK_PATTERN=^tyr
```

`NETWORK_PATTERN` is an optional environment variable containing the regex pattern that is used to locate all "required" networks.

> Do not connect **doneman** to any overlay networks: if it is not able to start, then the whole point is futile.

Doneman **requires a configuration file**, which needs to be copied inside the container.

So, after you spin up your whole stack, you need to do the following:

```bash
docker compose cp docker-compose.yml doneman:/docker-compose.yml
```

Doneman expects a `/docker-compose.yml` file in its root. Additionally you can copy `/.env` file if your docker compose relies on it, but make sure to copy it before the `/docker-compose.yml` file.

### Using in a Dockerfile

Additionally, if you cannot copy files inside the container for some reason - you can create your own image with the file already there. Here's an example of a Dockerfile:

```Dockerfile
FROM ewancoder/doneman
COPY containers.yml /config.yml

ENTRYPOINT ["/app"]
```

### Custom configuration

If for some reason your docker compose file is being read incorrectly, or you have unusual requirements - you can give Doneman a custom `containers.yml` file:

```bash
docker cp containers.yml my-doneman-container:/containers.yml
```

This way you can use it outside of the scope of any docker compose files.

An example of custom `containers.yml` file can be found in `app-example` folder of this repository.
