# Home Assistant

## Camera

Add the RTSP stream as a **Generic Camera**: in Home Assistant, **Settings → Devices & services → Add integration → Generic Camera**, one per camera. **Settings → Streaming** in the dashboard shows each camera's address under its Streaming Links, with a copy button.

- **Stream source URL:** `rtsp://192.168.1.100:8554/local/YOUR_BABY_UID`, or with the stream password on, `rtsp://nanit:KEY@192.168.1.100:8554/local/YOUR_BABY_UID`
- **RTSP transport protocol:** TCP, the only one the bridge serves
- **Still image URL:** `http://192.168.1.100:8080/api/snapshot/YOUR_BABY_UID`, plus `?key=KEY` with the stream password on. The bridge answers with a JPEG of the camera now, which is quicker and lighter than Home Assistant opening the stream for each still. Left empty, Home Assistant takes stills from the stream itself.

The snapshot address opens with a dashboard session, or with the stream password's key (`?key=KEY`, or basic auth as user `nanit`). With a dashboard password or SSO on, turn the stream password on so Home Assistant can use the key; behind a sign-in proxy, let `/api/snapshot/` through. With neither password on, it's open, like RTSP.

A new stream key means editing the address in the camera's options (**Configure** on the Generic Camera).

Notes:

- **Low latency:** Home Assistant 2024.11 and later plays RTSP cameras over WebRTC through its built-in go2rtc, under a second behind. Cards show it live with `camera_view: live`.
- **TCP only:** the bridge serves RTSP over TCP. An ffmpeg camera in `configuration.yaml` (`platform: ffmpeg`, `input:` the same address) also works: it tries UDP first, is refused, and switches straight away.
- **Home Assistant is a viewer of the stream server:** point it at the instance with `NANIT_STREAM_SERVER=true` (or at a viewer instance, which re-serves the same stream). It never asks the camera to stream, so it can't take the stream away from the bridge.
- **RTMP** (`rtmp://192.168.1.100:1935/local/YOUR_BABY_UID`, plus `?key=KEY` with the stream password on) also works, but goes through Home Assistant's slower HLS conversion.

## Sensors and switches

With `NANIT_MQTT_ENABLED=true`, readings are published over MQTT (see the [README](../README.md#mqtt) for every topic). Add them as MQTT entities:

```yaml
mqtt:
  sensor:
    - name: "Nanit Temperature"
      state_topic: "nanit/babies/YOUR_BABY_UID/temperature"
      device_class: temperature
      unit_of_measurement: "°C"
    - name: "Nanit Humidity"
      state_topic: "nanit/babies/YOUR_BABY_UID/humidity"
      device_class: humidity
      unit_of_measurement: "%"
  binary_sensor:
    - name: "Nanit Night Mode"
      state_topic: "nanit/babies/YOUR_BABY_UID/is_night"
      payload_on: "true"
      payload_off: "false"
  switch:
    - name: "Nanit Night Light"
      state_topic: "nanit/babies/YOUR_BABY_UID/night_light"
      command_topic: "nanit/babies/YOUR_BABY_UID/night_light/switch"
      payload_on: "true"
      payload_off: "false"
```

Your baby UIDs are shown in **Settings → Devices**.

## See also

- [Setup with NVR/Zoneminder](https://community.home-assistant.io/t/nanit-showing-in-ha-via-nvr-zoneminder/251641) by @jaburges
