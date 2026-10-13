# Nanit Web

A self-hosted bridge for Nanit baby monitors. It serves a web dashboard with live video, room readings, history and camera controls. It re-serves the camera's stream over RTSP and RTMP for Home Assistant, Frigate, VLC and other players, and publishes readings over MQTT.

It isn't affiliated with Nanit. It uses Nanit's unofficial, reverse-engineered API, which can change without notice.

## Features

- **Live video in the browser, under a second behind.** The camera's H.264/AAC stream goes to the dashboard as fragmented MP4 over a WebSocket and plays with Media Source Extensions (including iPhones on iOS 17.1 or later), with no re-encoding. Browsers that can't play it fall back to HLS, a few seconds behind. Playback is click-to-play, with sound, a volume control, snapshots and fullscreen.
- **RTSP and RTMP for other software.** The same stream is served over RTSP (Home Assistant, Frigate, go2rtc, NVRs) and RTMP (OBS, VLC). Several players can watch at once, and a slow one can't stall the rest.
- **One stream server, any number of viewers.** The camera can only stream to one place. One instance asks for it; other instances (a test copy on a laptop, say) take their video from that one instead of competing for the camera. See [Stream server and viewers](#stream-server-and-viewers).
- **Passwords on everything that streams.** An optional dashboard password covers the dashboard, its live view and HLS; an optional stream password covers RTSP and RTMP.
- **Readings.** Temperature, humidity, the camera's day/night mode, and night light state, updated live.
- **Controls.** Night light, standby, and the camera's built-in sounds (White Noise, Birds, Waves, Wind) with play/stop, a timer and speaker volume.
- **Device details.** Firmware, settings, and the camera's WiFi network, signal strength and channel.
- **History.** Temperature and humidity charts, and a day/night timeline that marks the time nothing was recorded, over 1 hour to 30 days, stored in SQLite.
- **Crying log, with recordings.** With [Frigate](https://frigate.video) listening to the stream for crying, each crying spell shows on the camera's card as it happens, with how long it lasted and its recording. See [Crying spells from Frigate](#crying-spells-from-frigate).
- **Dashboard.** Light and dark themes (Catppuccin Latte and Mocha, following the device or set by hand). Several cameras can be collapsed and reordered, per browser.
- **Sign-in.** Nanit's email/password and 2FA (text or email code) from the dashboard, plus an optional password on the dashboard itself.
- **MQTT.** Readings and switch topics for Home Assistant or anything else. See [MQTT](#mqtt).

## Quick start

```bash
docker run -d \
  --name nanit \
  --restart unless-stopped \
  -v /path/to/data:/data \
  -p 8080:8080 \
  -p 1935:1935 \
  -p 8554:8554 \
  -e NANIT_RTMP_ADDR=192.168.1.100:1935 \
  -e NANIT_STREAM_SERVER=true \
  ghcr.io/andrewshaodev/nanit-web:latest
```

1. Set `NANIT_RTMP_ADDR` to **this machine's LAN address** (or a host name that resolves to it on your network) and the RTMP port. The camera connects to it to send its stream, so it can't be `localhost`.
2. Keep `NANIT_STREAM_SERVER=true` on this, your main instance. **Without it, the instance is a viewer and has no video of its own.** Only one instance should have it.
3. Open `http://<host>:8080`. You'll be taken to the sign-in page.
4. Sign in with your Nanit email and password, then enter the code Nanit sends by text or email. The session is saved in `/data`, so you won't need to sign in again after a restart.
5. Optionally, set a dashboard password and a stream password under **Settings** (see [Passwords](#passwords)).

Ports: `8080` is the dashboard, `1935` RTMP (the camera pushes its stream here, and RTMP players watch), and `8554` RTSP.

For Docker Compose, see [docs/docker-compose.md](docs/docker-compose.md).

The image is built for `linux/amd64` from `main` by GitHub Actions. `latest` follows `main`; `main-<commit>` tags pin a specific build. The dashboard's footer shows the commit you're running.

## Configuration

Settings are environment variables, apart from the passwords and a viewer's stream server, which are set in the dashboard and kept in the data folder. [.env.sample](.env.sample) has the same list with comments.

| Variable | Default | Description |
|---|---|---|
| `NANIT_STREAM_SERVER` | `false` | This instance asks the cameras to stream. Set it on exactly one instance; the others are viewers |
| `NANIT_RTMP_ADDR` | *required* | Address and port the camera can reach this bridge on, e.g. `192.168.1.100:1935` or `monitor.home.lan:1935`. Its host is also used in the RTSP links |
| `NANIT_RTMP_ENABLED` | `true` | Run the built-in RTMP server |
| `NANIT_RTMP_AUTO_START` | `true` | Ask the camera to start streaming when it comes online, and again if the stream drops. With `false`, it streams only once the dashboard is opened. Stream server only |
| `NANIT_RTSP_ENABLED` | `true` | Serve each camera's stream over RTSP |
| `NANIT_RTSP_ADDR` | `:8554` | Where the RTSP server listens. TCP only, so a container needs just this one port |
| `NANIT_HTTP_PORT` | `8080` | Dashboard and API port |
| `NANIT_WEB_DIR` | `web` | The built dashboard, relative to the working directory (`/app/web` in the image) |
| `NANIT_DATA_DIR` | `/data` | Where the session, history database and other files are kept |
| `NANIT_SESSION_FILE` | `session.json` in the data dir | Saved Nanit session (contains tokens, so keep it private) |
| `NANIT_LOG_LEVEL` | `info` | `trace`, `debug`, `info`, `warn` or `error` |
| `NANIT_HISTORY_ENABLED` | `true` | Record readings for the history charts |
| `NANIT_HISTORY_RETENTION_DAYS` | `30` | How long history is kept |
| `NANIT_HISTORY_CLEANUP_ENABLED` | `true` | Delete history older than the retention period |
| `NANIT_MQTT_ENABLED` | `false` | Publish to an MQTT broker |
| `NANIT_MQTT_BROKER_URL` | | Broker URL, e.g. `tcp://homeassistant:1883` |
| `NANIT_MQTT_USERNAME` / `NANIT_MQTT_PASSWORD` | | Broker credentials |
| `NANIT_MQTT_CLIENT_ID` | `nanit` | MQTT client ID |
| `NANIT_MQTT_PREFIX` | `nanit` | Topic prefix |
| `NANIT_FRIGATE_URL` | | Frigate's API, e.g. `http://frigate:5000`, to show crying spells (see [Crying spells from Frigate](#crying-spells-from-frigate)) |
| `NANIT_FRIGATE_CAMERAS` | | Frigate camera names and the babies they watch, e.g. `nursery=bb36bfe3,travel=380fb4f2` |
| `NANIT_FRIGATE_PREFIX` | `frigate` | Frigate's MQTT topic prefix |
| `NANIT_EVENTS_POLLING` | `false` | Poll Nanit for motion and sound events (see [MQTT](#mqtt)) |
| `NANIT_EVENTS_POLLING_INTERVAL` | `30` | Seconds between polls |
| `NANIT_EVENTS_MESSAGE_TIMEOUT` | `300` | Ignore events older than this many seconds |
| `NANIT_REFRESH_TOKEN` | | Start from an existing Nanit refresh token instead of signing in |
| `NANIT_EMAIL` / `NANIT_PASSWORD` | | Used to sign in again if the saved session expires. They can't get past a 2FA prompt, so sign in from the dashboard first. |

### Running behind a reverse proxy

The dashboard works over HTTPS behind a reverse proxy (Caddy, Traefik, nginx, Pangolin...). Every URL it uses is relative to the page, so there are no mixed-content problems. Proxy the HTTP port, and let WebSockets through: the live view uses one at `/api/stream/live/...` (if they're blocked, the dashboard falls back to HLS and says so under the video). The RTMP and RTSP ports are plain TCP on your LAN, so leave them out of the proxy.

### Stream server and viewers

A Nanit camera pushes its stream to one address only. If two bridges both asked it to stream to themselves, they would keep taking the stream from each other. So:

- **The stream server** (`NANIT_STREAM_SERVER=true`) is the only instance that asks the cameras to stream. Everything else gets the video from it.
- **A viewer** (any other instance) never asks the cameras to stream. It takes the server's RTSP stream and re-serves it: its own dashboard, live view, HLS, RTSP and RTMP work as on the server. Set where it gets the video in its dashboard, under **Settings → Streaming → Stream server**: the server's host, RTSP port (8554) and, if the server has a stream password, its key. Saving reconnects straight away, and each camera's line says whether video is arriving, or why not.

Viewers still keep their own connection to Nanit for readings and controls, and keep their own history.

If two instances are set as the stream server by mistake, the server notices its stream being taken repeatedly, shows an alert on the camera ("Another bridge may be taking this camera's stream"), and asks far less often until it stops.

### Passwords

There are two, for two kinds of access. Both are off until you turn them on.

- **Dashboard password** (**Settings → Authentication & Security → Require a password**). People opening the dashboard in a browser sign in with it. It covers the dashboard and its whole API, including the live view, HLS, controls, history and Nanit sign-in. Only the page itself, its login endpoints and the `/health` and `/ready` checks stay open. Turning it on asks for the password; **Generate a strong password** makes one for you.
- **Stream password** (**Settings → Streaming → Stream password → Require a password**). Programs that play the stream use it: Home Assistant, Frigate, VLC, viewer instances. Turning it on generates a key. RTSP players sign in as user `nanit` with the key as the password (`rtsp://nanit:KEY@host:8554/local/<baby_uid>`); RTMP players add `?key=KEY` to the address. The Streaming Links and the Home Assistant example on that page include it, masked on screen and complete when copied. **New key** replaces it. Turning it on or off, or making a new key, disconnects players already watching. The camera's own push to the bridge never needs the key, so changing it can't stop the stream.

The dashboard password protects the stream key too: without it, anyone who can open the dashboard can read the key.

To remove a forgotten dashboard password:

```bash
docker exec -it nanit /app/bin/nanit --reset-password
```

## Home Assistant

### Camera

Add the RTSP stream as an ffmpeg camera. **Settings → Streaming** shows each camera's exact address under its Streaming Links, and a ready-made example with a copy button:

```yaml
camera:
  - platform: ffmpeg
    name: Nanit
    input: rtsp://192.168.1.100:8554/local/YOUR_BABY_UID
    # with the stream password on:
    # input: rtsp://nanit:KEY@192.168.1.100:8554/local/YOUR_BABY_UID
```

Home Assistant 2024.11 and later plays RTSP cameras over WebRTC with its built-in go2rtc, under a second behind. The bridge serves RTSP over TCP only; ffmpeg and go2rtc switch to TCP on their own. RTMP (`rtmp://192.168.1.100:1935/local/YOUR_BABY_UID`) still works, but goes through Home Assistant's slower HLS conversion. See also [docs/home-assistant.md](docs/home-assistant.md).

### MQTT

With `NANIT_MQTT_ENABLED=true`, readings are published to `<prefix>/babies/<baby_uid>/<name>`:

| Topic name | Value |
|---|---|
| `temperature` | °C, e.g. `23.1` |
| `humidity` | %, e.g. `54.2` |
| `is_night` | `true` when the camera is in night mode |
| `night_light` | `true` / `false` |
| `standby` | `true` / `false` |
| `is_stream_alive` | `true` while the camera is streaming to the bridge |
| `motion_timestamp` / `sound_timestamp` | Unix time of the latest motion or sound event (needs `NANIT_EVENTS_POLLING=true` and those notifications turned on in the Nanit app) |

Publish `true` or `false` to `<prefix>/babies/<baby_uid>/night_light/switch` or `.../standby/switch` to control them.

This build doesn't send Home Assistant MQTT discovery messages, so add these as [MQTT sensors and switches](https://www.home-assistant.io/integrations/sensor.mqtt/) in your configuration. Your baby UIDs are shown in **Settings → Devices**.

### Crying spells from Frigate

[Frigate](https://frigate.video) can watch the bridge's RTSP output and listen for crying with its audio detection. The bridge then keeps a log of each crying spell, and the camera's card shows it: when each spell started, how long it lasted, whether the baby is crying now, and the recording.

In Frigate, listen for `crying` on the camera, make it an alert (`review.alerts.labels: [crying]`) and record alerts. Then set:

- `NANIT_FRIGATE_URL` to Frigate's API. Port 5000 has no password, so reach it only over a Docker network, never published.
- `NANIT_FRIGATE_CAMERAS` to which Frigate camera is which baby. The baby UIDs are in **Settings → Devices**.
- MQTT, on the same broker as Frigate (`NANIT_MQTT_*`), so spells show up as they happen. The broker user needs to read `frigate/reviews`. Without MQTT, the bridge checks Frigate every 5 minutes instead.

Each spell is one of Frigate's review items. The bridge keeps the times in its history database, as long as the rest of the history (`NANIT_HISTORY_RETENTION_DAYS`), and asks Frigate for the recording when it's played, from 30 s before the first cry to 30 s after the last. Recordings last only as long as Frigate keeps them. The history database must be on (`NANIT_HISTORY_ENABLED`).

## Troubleshooting

### The video won't load in the dashboard

The bridge only has video while the camera is streaming to it. On the stream server, check the logs (`docker logs nanit`) for `New stream publisher connected`. If it's missing:

- Check the instance has `NANIT_STREAM_SERVER=true`. Without it, it never asks the camera to stream (the log says `Viewer:` at startup).
- Check `NANIT_RTMP_ADDR` is an address the camera can reach, and that the RTMP port is published. A host name works if the camera can resolve it (it uses your router's DNS). A camera on a different network (a travel router, say) can't stream to the bridge.

On a viewer, **Settings → Streaming → Stream server** says what's wrong for each camera: a missing or wrong key, a host that doesn't resolve, nothing listening on that port, or a server that isn't receiving that camera.

If the video plays but sits a few seconds behind, the dashboard has fallen back to HLS; a line under the video says why. Usually a proxy is blocking WebSockets.

### "Number of Mobile App connections above limit"

Nanit limits how many app connections an account can have at once, and every bridge instance counts as one, viewers included. Close the Nanit app on a device or two, or wait a few minutes: the bridge asks again after a minute, then less often, up to every 15 minutes, until the camera streams.

### A player (VLC, ffmpeg, Home Assistant) can't open the RTSP or RTMP address

1. **Copy the address from Settings → Streaming → Streaming Links.** It uses the host from `NANIT_RTMP_ADDR` and includes the stream password's key when one is set.
2. **`401 Unauthorized`, or RTMP closing straight away:** the stream password is on and the key is missing or wrong. The logs say `RTMP player refused: missing or wrong stream password key` for RTMP.
3. **`404 Not Found` (RTSP), or "unable to open the MRL" (VLC with RTMP):** the camera isn't streaming to the bridge yet. Keep `NANIT_RTMP_AUTO_START=true` on the stream server, or open the dashboard, which asks for the stream, and check the logs for `New stream publisher connected`.

### Logs

```bash
docker logs -f nanit        # follow
docker logs --tail 100 nanit
```

Passwords, tokens and 2FA codes are never written to the log.

## Development

Requirements: Go 1.27, [Bun](https://bun.sh) 1.4, and ffmpeg for HLS.

```bash
# Backend (serves ./web as the dashboard, so build the frontend into it first)
cd frontend && bun install && bun run build && cd ..
mkdir -p web && cp -R frontend/dist/* web/
# As a viewer of your running stream server: set its address in the
# dashboard under Settings → Streaming → Stream server
NANIT_RTMP_ADDR=localhost:1935 NANIT_DATA_DIR=./data NANIT_HTTP_PORT=18089 go run ./cmd/nanit

# Frontend with hot reload, proxying /api to the backend above
cd frontend && bun run dev

# Checks (CI runs these before building the image)
go vet ./... && go test -race ./...
cd frontend && bun run typecheck && bun run lint

# Container image
docker build -t nanit-web .
```

The frontend's API types in `frontend/src/types/generated/` are generated from the Go structs the API sends (`pkg/httpapi/apitypes` and the `types.go` files it uses). Change the Go types, then run `go generate ./pkg/httpapi/apitypes`; CI fails if the generated files are out of date.

Run a local copy as a viewer (no `NANIT_STREAM_SERVER`), and it takes its video from your stream server without disturbing it. Give it its own data folder and HTTP port. Only run a second stream server if the first is stopped.

Where things are:

| Package | What it does |
|---|---|
| `cmd/nanit` | Reads the configuration and runs the app |
| `pkg/app` | Starts and stops everything, in order |
| `pkg/camera` | One camera: its connection to Nanit, its stream requests (stream server only), and its commands (night light, standby, sounds) |
| `pkg/httpapi` | The dashboard and its API, including the live view's WebSocket. `apitypes` has every response, and `testdata/golden` the JSON the dashboard expects |
| `pkg/client` | Nanit's REST API, sign-in, and the camera websocket (`websocket.proto`) |
| `pkg/relay` | Each camera's live stream, fanned out to every output. The camera (or a viewer's pull) publishes into it |
| `pkg/rtmpserver` | RTMP: the camera publishes here, and RTMP players watch |
| `pkg/rtspserver` | RTSP output |
| `pkg/pull` | A viewer's pull of the stream server's RTSP stream |
| `pkg/mse` | Fragmented MP4 for the dashboard's live view |
| `pkg/streamauth` / `pkg/streamsource` | The stream password, and a viewer's stream server setting |
| `pkg/streaming` | ffmpeg's HLS remux, the dashboard's fallback |
| `pkg/history` | The SQLite history |
| `pkg/mqtt` | MQTT readings and switches |
| `frontend` | The React dashboard |

The frontend is React 19 with Vite, Tailwind CSS 4, shadcn/ui on the Catppuccin theme, and Lucide icons. See [frontend/README.md](frontend/README.md).

## History and credits

This project grew out of several before it:

- [adam.stanek/nanit](https://gitlab.com/adam.stanek/nanit): the original reverse engineering of Nanit's API and camera protocol
- [indiefan/home_assistant_nanit](https://github.com/indiefan/home_assistant_nanit): the Home Assistant bridge this is built on
- [daleiii/nanit-web](https://github.com/daleiii/nanit-web): added the web dashboard, 2FA sign-in, HLS video and history
- [maddijoyce/nanit-web](https://github.com/maddijoyce/nanit-web): GHCR builds, stream and token-renewal fixes, and the decoding of the camera's sound and RTMP-address APIs, which this fork cherry-picks

This fork adds a rebuilt frontend, a sub-second live view, RTSP output, a stream server with viewer instances, stream and dashboard passwords, the sound controls, the camera's WiFi status, a restructured backend (a camera object per camera, streaming that follows the camera, a typed API), and a long list of fixes. The fixes include stream requests that could empty the data folder, a crash when two browsers signed in at once, MQTT commands going to the wrong camera, HLS retries that never ran, day/night history inventing data, credentials in the logs, TLS verification on the camera connection, an RTMP relay that stalled on slow viewers, and an API that mostly skipped the dashboard password. `git log` has the details.

## Security

This bridge holds a session with full access to your Nanit account, live video from your child's room, and controls for the camera. Keep it on your home network or behind a reverse proxy with real authentication, don't expose it directly to the internet, and keep the `/data` directory private.

- **Turn on both passwords** (see [Passwords](#passwords)), so every way the video leaves the bridge needs one. A dashboard behind an SSO proxy is still reachable directly by anything sharing its Docker network.
- **RTSP accepts Basic sign-in as well as Digest,** and RTMP's key travels in the address, so neither protects against someone watching your network traffic. They stop other devices on your network from casually watching.
- **The camera's push into port 1935 has no password**, so a device on your network could push its own video in its place. If that matters to you, allow port 1935 on the stream server's host only from the camera's IP address (Docker's published ports bypass ufw; put the rule in the `DOCKER-USER` chain). That also shuts out RTMP players, so watch over RTSP instead.

## Disclaimer and license

This is a personal project, provided as is and without warranty, for personal use. You're responsible for securing your installation, and for how you use it.

Licensed under the MIT License. See [LICENSE](LICENSE).
