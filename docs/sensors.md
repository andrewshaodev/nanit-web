# Sensors

The bridge publishes each camera's readings over MQTT. See the `NANIT_MQTT_*` variables in [.env.sample](../.env.sample) to turn it on, and the [README](../README.md#mqtt) for every topic, including the night light and standby switches.

The main ones, under `nanit/babies/{baby_uid}/` (the prefix is `NANIT_MQTT_PREFIX`):

- `temperature`: degrees Celsius (float)
- `humidity`: percent (float)
- `is_night`: whether the camera is in night mode (bool)

To use them in Home Assistant, see [home-assistant.md](./home-assistant.md). If something doesn't arrive, [MQTT Explorer](http://mqtt-explorer.com/) shows what is being published.
