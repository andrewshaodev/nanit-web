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
      - "1935:1935"   # RTMP: the camera pushes its stream here, and RTMP players watch
      - "8554:8554"   # RTSP, for Home Assistant, Frigate and viewer instances
    environment:
      NANIT_STREAM_SERVER: "true"             # this is the instance that asks the cameras to stream
      NANIT_RTMP_ADDR: "192.168.1.100:1935"   # this host's LAN address (or a name for it), reachable by the camera
      TZ: "America/New_York"
      # Optional: publish readings to Home Assistant's broker
      # NANIT_MQTT_ENABLED: "true"
      # NANIT_MQTT_BROKER_URL: "tcp://homeassistant:1883"
      # NANIT_MQTT_USERNAME: "nanit"
      # NANIT_MQTT_PASSWORD: "..."
      # Optional: crying spells from Frigate, on the same Docker network
      # NANIT_FRIGATE_URL: "http://frigate:5000"
      # NANIT_FRIGATE_CAMERAS: "nursery=<baby_uid>"
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

Then open `http://<host>:8080` and sign in to Nanit. The session is kept in `./data`. Under **Settings**, you can then turn on the dashboard password and the stream password.

To publish a port on one interface only, prefix it with the address, e.g. `"192.168.1.100:8554:8554"`.

## A second instance as a viewer

A second instance (a test copy on another machine, say) leaves out `NANIT_STREAM_SERVER`, so it never asks the cameras to stream. It needs its own data folder, and `NANIT_RTMP_ADDR` is just the host shown in its own links (`localhost:1935` is fine). Then, in its dashboard under **Settings → Streaming → Stream server**, enter the stream server's host, RTSP port (8554) and, if the server's stream password is on, its key.

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

Point the proxy at port 8080 (or the container on a shared Docker network) and drop the `8080` port mapping. Let WebSockets through, for the live view. Keep `1935` and `8554` published: the camera and RTSP players connect to them directly on your LAN. The dashboard works over HTTPS as-is.

Other containers on the proxy's Docker network can reach port 8080 without going through the proxy's sign-on, so set a dashboard password as well.
