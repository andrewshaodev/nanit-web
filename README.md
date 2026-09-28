# Nanit Web

A self-hosted bridge for Nanit baby monitors. It serves a web dashboard with live video, room readings, history and camera controls. It relays the camera's stream over RTMP for Home Assistant, VLC and other players, and publishes readings over MQTT.

It isn't affiliated with Nanit. It uses Nanit's unofficial, reverse-engineered API, which can change without notice.

## Features

- **Live video in the browser.** The camera's H.264/AAC stream is remuxed to HLS without re-encoding (about 1% CPU), and it works over HTTPS behind a reverse proxy.
- **RTMP relay.** The same stream is served over RTMP for Home Assistant, VLC and anything else that plays RTMP. It copes with several viewers, and a slow one can't stall the rest.
- **Readings.** Temperature, humidity, the camera's day/night mode, and night light state, updated live.
- **Controls.** Night light, standby, and the camera's built-in sounds (White Noise, Birds, Waves, Wind) with play/stop, a timer and speaker volume.
- **Device details.** Firmware, settings, and the camera's WiFi network, signal strength and channel.
- **History.** Temperature and humidity charts, and a day/night timeline that marks the time nothing was recorded, over 1 hour to 30 days, stored in SQLite.
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
  -e NANIT_RTMP_ADDR=192.168.1.100:1935 \
  ghcr.io/andrewshaodev/nanit-web:latest
```

1. Set `NANIT_RTMP_ADDR` to **this machine's LAN address** and the RTMP port. The camera connects to it to send its stream, so it can't be `localhost`.
2. Open `http://<host>:8080`. You'll be taken to the sign-in page.
3. Sign in with your Nanit email and password, then enter the code Nanit sends by text or email. The session is saved in `/data`, so you won't need to sign in again after a restart.

For Docker Compose, see [docs/docker-compose.md](docs/docker-compose.md).

The image is built for `linux/amd64` from `main` by GitHub Actions. `latest` follows `main`; `main-<commit>` tags pin a specific build. The dashboard's footer shows the commit you're running.

## Configuration

All settings are environment variables. [.env.sample](.env.sample) has the same list with comments.

| Variable | Default | Description |
|---|---|---|
| `NANIT_RTMP_ADDR` | *required* | Address and port the camera can reach this bridge on, e.g. `192.168.1.100:1935` |
| `NANIT_RTMP_ENABLED` | `true` | Run the built-in RTMP server |
| `NANIT_RTMP_AUTO_START` | `true` | Ask the camera to start streaming when it comes online, and again if the stream drops. With `false`, it streams only once the dashboard is opened |
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
| `NANIT_EVENTS_POLLING` | `false` | Poll Nanit for motion and sound events (see [MQTT](#mqtt)) |
| `NANIT_EVENTS_POLLING_INTERVAL` | `30` | Seconds between polls |
| `NANIT_EVENTS_MESSAGE_TIMEOUT` | `300` | Ignore events older than this many seconds |
| `NANIT_REFRESH_TOKEN` | | Start from an existing Nanit refresh token instead of signing in |
| `NANIT_EMAIL` / `NANIT_PASSWORD` | | Used to sign in again if the saved session expires. They can't get past a 2FA prompt, so sign in from the dashboard first. |

### Running behind a reverse proxy

The dashboard works over HTTPS behind a reverse proxy (Caddy, Traefik, nginx, Pangolin...). Every URL it uses is relative to the page, so there are no mixed-content problems. Proxy the HTTP port. The RTMP port is plain TCP and is used on your LAN, so leave it out of the proxy.

### Dashboard password

You can set a password in **Settings → Authentication & Security**. With one set, the dashboard and the whole API (video, controls, history, Nanit sign-in) require it. Only the page itself, its login endpoints and the `/health` and `/ready` checks stay open. If you already protect the dashboard at the proxy with SSO, you don't need both.

To remove a forgotten password:

```bash
docker exec -it nanit /app/bin/nanit --reset-password
```

## Home Assistant

### Camera

Add the RTMP stream as an ffmpeg camera. The dashboard shows each camera's exact URL under **Settings → Streaming → Streaming Links**:

```yaml
camera:
  - platform: ffmpeg
    name: Nanit
    input: rtmp://192.168.1.100:1935/local/YOUR_BABY_UID
```

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

## Troubleshooting

### The video won't load in the dashboard

The bridge only has video while the camera is streaming to it. Check the logs (`docker logs nanit`) for `New stream publisher connected`. If it's missing, check `NANIT_RTMP_ADDR` is an address the camera can reach, and that the RTMP port is published. A camera on a different network (a travel router, say) can't stream to the bridge.

### "Number of Mobile App connections above limit"

Nanit limits how many app connections an account can have at once, and the bridge counts as one. Close the Nanit app on a device or two, or wait a few minutes: the bridge asks again after a minute, then less often, up to every 15 minutes, until the camera streams. Don't run two bridges on the same account: they compete for the cameras.

### VLC (or ffplay, or Home Assistant) can't open the RTMP URL

1. **Use the address from `NANIT_RTMP_ADDR`**, not the dashboard's address. Copy it from Settings → Streaming → Streaming Links, which reads it from the server.
2. **The camera has to be streaming first.** The RTMP server is a relay: it closes viewers that connect while the camera isn't publishing, which VLC reports as "unable to open the MRL". Keep `NANIT_RTMP_AUTO_START=true`, or open the dashboard, which asks for the stream, and check the logs for `New stream publisher connected`.

### Logs

```bash
docker logs -f nanit        # follow
docker logs --tail 100 nanit
```

Passwords, tokens and 2FA codes are never written to the log.

## Development

Requirements: Go 1.27, [Bun](https://bun.sh) 1.3, and ffmpeg for HLS.

```bash
# Backend (serves ./web as the dashboard, so build the frontend into it first)
cd frontend && bun install && bun run build && cd ..
mkdir -p web && cp -R frontend/dist/* web/
NANIT_RTMP_ADDR=<your LAN IP>:1935 NANIT_DATA_DIR=./data go run ./cmd/nanit

# Frontend with hot reload, proxying /api to the backend above
cd frontend && bun run dev

# Checks (CI runs these before building the image)
go vet ./... && go test -race ./...
cd frontend && bun run typecheck && bun run lint

# Container image
docker build -t nanit-web .
```

The frontend's API types in `frontend/src/types/generated/` are generated from the Go structs the API sends (`pkg/httpapi/apitypes` and the `types.go` files it uses). Change the Go types, then run `go generate ./pkg/httpapi/apitypes`; CI fails if the generated files are out of date.

Only one bridge should be connected to your cameras at a time. Stop any other instance before running one locally.

Where things are:

| Package | What it does |
|---|---|
| `cmd/nanit` | Reads the configuration and runs the app |
| `pkg/app` | Starts and stops everything, in order |
| `pkg/camera` | One camera: its connection to Nanit, its stream, and its commands (night light, standby, sounds) |
| `pkg/httpapi` | The dashboard and its API. `apitypes` has every response, and `testdata/golden` the JSON the dashboard expects |
| `pkg/client` | Nanit's REST API, sign-in, and the camera websocket (`websocket.proto`) |
| `pkg/rtmpserver` | The RTMP relay the cameras publish to |
| `pkg/streaming` | ffmpeg's HLS remux for the dashboard |
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

This fork adds a rebuilt frontend, the sound controls, the camera's WiFi status, a restructured backend (a camera object per camera, streaming that follows the camera, a typed API), and a long list of fixes. The fixes include stream requests that could empty the data folder, a crash when two browsers signed in at once, MQTT commands going to the wrong camera, HLS retries that never ran, day/night history inventing data, credentials in the logs, TLS verification on the camera connection, an RTMP relay that stalled on slow viewers, and an API that mostly skipped the dashboard password. `git log` has the details.

## Security

This bridge holds a session with full access to your Nanit account, live video from your child's room, and controls for the camera. Keep it on your home network or behind a reverse proxy with real authentication, don't expose it directly to the internet, and keep the `/data` directory private.

## Disclaimer and license

This is a personal project, provided as is and without warranty, for personal use. You're responsible for securing your installation, and for how you use it.

Licensed under the MIT License. See [LICENSE](LICENSE).
