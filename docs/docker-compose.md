# Docker Compose

Create a `docker-compose.yml`:

```yaml
services:
  nanit:
    image: ghcr.io/andrewshaodev/nanit-web:latest
    # Check for a newer image on every deploy (see "Upgrading" below)
    pull_policy: always
    container_name: nanit
    restart: unless-stopped
    ports:
      - "8080:8080"   # dashboard; leave this out if a reverse proxy reaches it over a Docker network
      - "1935:1935"   # RTMP, which the camera connects to
    environment:
      NANIT_RTMP_ADDR: "192.168.1.100:1935"   # this host's LAN address, reachable by the camera
      TZ: "America/New_York"
      # Optional: publish readings to Home Assistant's broker
      # NANIT_MQTT_ENABLED: "true"
      # NANIT_MQTT_BROKER_URL: "tcp://homeassistant:1883"
      # NANIT_MQTT_USERNAME: "nanit"
      # NANIT_MQTT_PASSWORD: "..."
    volumes:
      - ./data:/data
    healthcheck:
      test: ["CMD", "curl", "-sf", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 5
      start_period: 20s
```

See [.env.sample](../.env.sample) for every option. Prefer `NANIT_MQTT_USERNAME` and `NANIT_MQTT_PASSWORD` to putting credentials in the broker URL.

Then open `http://<host>:8080` and sign in to Nanit. The session is kept in `./data`.

## Everyday commands

```bash
docker compose up -d        # start
docker compose logs -f      # follow the logs
docker compose down         # stop and remove the container (./data is kept)
```

## Upgrading

`latest` is rebuilt from `main`. With `pull_policy: always`, every `docker compose up -d` checks for a newer image and recreates the container if there is one. A plain restart (`docker compose restart`, a crash, a reboot) doesn't pull.

To upgrade only when you choose, pin a build instead, for example `ghcr.io/andrewshaodev/nanit-web:main-8de69d8`, and change the tag to move to a newer one.

## Behind a reverse proxy

Point the proxy at port 8080 (or the container on a shared Docker network) and drop the `8080` port mapping. Keep `1935` published: the camera connects to it directly on your LAN. The dashboard works over HTTPS as-is.
