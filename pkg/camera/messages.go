package camera

// Turning the camera's websocket messages into state updates

import (
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/client"
	"github.com/rs/zerolog/log"
)

func processSensorData(babyUID string, sensorData []*client.SensorData, stateManager *baby.StateManager) {
	// Parse sensor update
	stateUpdate := baby.State{}
	for _, sensorDataSet := range sensorData {
		if *sensorDataSet.SensorType == client.SensorType_TEMPERATURE {
			stateUpdate.SetTemperatureMilli(*sensorDataSet.ValueMilli)
		}
		if *sensorDataSet.SensorType == client.SensorType_HUMIDITY {
			stateUpdate.SetHumidityMilli(*sensorDataSet.ValueMilli)
		}
		if *sensorDataSet.SensorType == client.SensorType_NIGHT {
			stateUpdate.SetIsNight(*sensorDataSet.Value == 1)
		}
	}

	stateManager.Update(babyUID, stateUpdate)
}

func processLight(babyUID string, control *client.Control, stateManager *baby.StateManager) {
	if control.NightLight != nil {
		stateUpdate := baby.State{}
		stateUpdate.SetNightLight(*control.NightLight == client.Control_LIGHT_ON)
		stateManager.Update(babyUID, stateUpdate)
	}
}

func processStandby(babyUID string, settings *client.Settings, stateManager *baby.StateManager) {
	stateUpdate := baby.State{}
	deviceInfo := &baby.DeviceInfo{}

	// Extract standby mode
	if settings.SleepMode != nil {
		stateUpdate.SetStandby(*settings.SleepMode)
		deviceInfo.SleepMode = settings.SleepMode
	}

	// Extract other device configuration
	if settings.NightVision != nil {
		deviceInfo.NightVision = settings.NightVision
	}
	if settings.Volume != nil {
		deviceInfo.Volume = settings.Volume
	}
	if settings.StatusLightOn != nil {
		deviceInfo.StatusLight = settings.StatusLightOn
	}
	if settings.MicMuteOn != nil {
		deviceInfo.MicMute = settings.MicMuteOn
	}
	if settings.AntiFlicker != nil {
		antiFlicker := ""
		switch *settings.AntiFlicker {
		case client.Settings_FR50HZ:
			antiFlicker = "50Hz"
		case client.Settings_FR60HZ:
			antiFlicker = "60Hz"
		default:
			antiFlicker = "Unknown"
		}
		deviceInfo.AntiFlicker = &antiFlicker
	}
	if settings.WifiBand != nil {
		wifiBand := ""
		switch *settings.WifiBand {
		case client.Settings_ANY:
			wifiBand = "Any"
		case client.Settings_FR2_4GHZ:
			wifiBand = "2.4GHz"
		case client.Settings_FR5_0GHZ:
			wifiBand = "5.0GHz"
		default:
			wifiBand = "Unknown"
		}
		deviceInfo.WiFiBand = &wifiBand
	}
	if settings.MountingMode != nil {
		mountingMode := int32(*settings.MountingMode)
		deviceInfo.MountingMode = &mountingMode
	}

	// Extract sensor thresholds
	for _, sensor := range settings.Sensors {
		if sensor.SensorType != nil {
			switch *sensor.SensorType {
			case client.SensorType_TEMPERATURE:
				if sensor.LowThreshold != nil {
					deviceInfo.TempLowThreshold = sensor.LowThreshold
				}
				if sensor.HighThreshold != nil {
					deviceInfo.TempHighThreshold = sensor.HighThreshold
				}
			case client.SensorType_HUMIDITY:
				if sensor.LowThreshold != nil {
					deviceInfo.HumidityLowThreshold = sensor.LowThreshold
				}
				if sensor.HighThreshold != nil {
					deviceInfo.HumidityHighThreshold = sensor.HighThreshold
				}
			}
		}
	}

	// Extract stream settings
	for _, stream := range settings.Streams {
		if stream.Id != nil {
			switch *stream.Id {
			case client.StreamIdentifier_MOBILE:
				if stream.Bitrate != nil {
					deviceInfo.MobileBitrate = stream.Bitrate
				}
				if stream.BestFps != nil {
					deviceInfo.MobileFPS = stream.BestFps
				}
			case client.StreamIdentifier_DVR:
				if stream.Bitrate != nil {
					deviceInfo.DVRBitrate = stream.Bitrate
				}
				if stream.BestFps != nil {
					deviceInfo.DVRFPS = stream.BestFps
				}
			case client.StreamIdentifier_ANALYTICS:
				if stream.Bitrate != nil {
					deviceInfo.AnalyticsBitrate = stream.Bitrate
				}
				if stream.BestFps != nil {
					deviceInfo.AnalyticsFPS = stream.BestFps
				}
			}
		}
	}

	// Set last updated timestamp
	timestamp := time.Now().Unix()
	deviceInfo.LastUpdated = &timestamp

	// Set device info in state
	stateUpdate.DeviceInfo = deviceInfo
	stateManager.Update(babyUID, stateUpdate)

	log.Debug().Str("baby_uid", babyUID).Interface("device_info", deviceInfo).Msg("Updated device info from settings")
}

// processNetwork - the camera's WiFi connection, from GET_STATUS_NETWORK
func processNetwork(babyUID string, network *client.NetworkStatus, stateManager *baby.StateManager) {
	deviceInfo := &baby.DeviceInfo{}
	if network.Ssid != nil {
		deviceInfo.WiFiNetwork = network.Ssid
	}
	if network.Rssi != nil {
		deviceInfo.WiFiSignal = network.Rssi
	}
	if network.Frequency != nil {
		deviceInfo.WiFiFrequency = network.Frequency
	}
	stateManager.Update(babyUID, baby.State{DeviceInfo: deviceInfo})
}

func processStatus(babyUID string, status *client.Status, stateManager *baby.StateManager) {
	stateUpdate := baby.State{}
	deviceInfo := &baby.DeviceInfo{}

	// Extract device information from status
	if status.CurrentVersion != nil {
		deviceInfo.FirmwareVersion = status.CurrentVersion
	}
	if status.HardwareVersion != nil {
		deviceInfo.HardwareVersion = status.HardwareVersion
	}
	if status.Mode != nil {
		mode := ""
		switch *status.Mode {
		case client.MountingMode_STAND:
			mode = "Stand"
		case client.MountingMode_TRAVEL:
			mode = "Travel"
		case client.MountingMode_SWITCH:
			mode = "Switch"
		default:
			mode = "Unknown"
		}
		deviceInfo.DeviceMode = &mode
	}
	if status.UpgradeDownloaded != nil {
		deviceInfo.UpgradeDownloaded = status.UpgradeDownloaded
	}

	// Set last updated timestamp
	timestamp := time.Now().Unix()
	deviceInfo.LastUpdated = &timestamp

	// Set device info in state
	stateUpdate.DeviceInfo = deviceInfo
	stateManager.Update(babyUID, stateUpdate)

	log.Debug().Str("baby_uid", babyUID).Interface("device_info", deviceInfo).Msg("Updated device info from status")
}
