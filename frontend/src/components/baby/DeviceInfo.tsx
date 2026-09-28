import { useState } from 'react'
import useSWR from 'swr'
import { api } from '@/lib/api'
import type { Baby, DeviceInfoResponse } from '@/types/api'
import {
  AlertTriangle,
  ChevronRight,
  CircleX,
  Lightbulb,
  Smartphone,
  Video,
  Wifi,
  Wrench,
} from 'lucide-react'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

interface DeviceInfoProps {
  baby: Baby
}

export default function DeviceInfo({ baby }: DeviceInfoProps) {
  const [isExpanded, setIsExpanded] = useState(false)

  const { data, error, isLoading } = useSWR<DeviceInfoResponse>(
    isExpanded ? `/device-info/${baby.uid}` : null,
    () => api.getDeviceInfo(baby.uid),
    {
      revalidateOnFocus: false,
    }
  )

  // Debug logging
  if (data) {
    console.log('DeviceInfo API Response:', data)
    console.log('device_info object:', data.device_info)
    console.log('connection_status object:', data.connection_status)
  }

  const alerts = data?.alerts || []
  const hasAlerts = alerts.length > 0

  return (
    // A disclosure row, flush inside its CameraBox
    <div>
      <Button
        variant="ghost"
        onClick={() => setIsExpanded(!isExpanded)}
        aria-expanded={isExpanded}
        className="w-full h-auto px-4 py-2 rounded-none justify-between text-left font-normal hover:bg-accent aria-expanded:bg-transparent aria-expanded:hover:bg-accent"
      >
        <h3 className="font-medium flex items-center gap-2 text-muted-foreground">
          <Wrench />
          Device Details & Troubleshooting
          {hasAlerts && (
            <Badge variant="destructive">
              {alerts.length}
            </Badge>
          )}
        </h3>
        <ChevronRight className={`transition-transform duration-200 ${isExpanded ? 'rotate-90' : ''}`} />
      </Button>

      {isExpanded && (
        <div className="border-t p-4">
          {isLoading ? (
            <div className="flex items-center justify-center py-8">
              <LoadingSpinner size="md" />
            </div>
          ) : error ? (
            <div className="text-center py-8 text-destructive">
              <span className="inline-flex items-center gap-2">
                <CircleX className="size-4" />
                Failed to load device information
              </span>
              <Button
                variant="link"
                size="sm"
                onClick={() => setIsExpanded(false)}
                className="flex mx-auto mt-2 text-destructive"
              >
                Retry
              </Button>
            </div>
          ) : data ? (
            <div className="space-y-6">
              {/* Status Summary */}
              <div className="grid grid-cols-1 md:grid-cols-4 gap-4 p-4 bg-muted rounded-lg">
                <div className="text-center">
                  <div className="text-sm text-muted-foreground mb-1">Firmware</div>
                  <div className="font-semibold">
                    {data.device_info?.firmware_version || '--'}
                  </div>
                </div>
                <div className="text-center">
                  <div className="text-sm text-muted-foreground mb-1">Connection</div>
                  <div className="font-semibold">
                    {data.connection_status?.websocket_alive ? 'Connected' : 'Disconnected'}
                  </div>
                </div>
                <div className="text-center">
                  <div className="text-sm text-muted-foreground mb-1">Streaming</div>
                  <div className="font-semibold">
                    {data.device_info?.streaming_error ? 'Error' : 'Ready'}
                  </div>
                </div>
                <div className="text-center">
                  <div className="text-sm text-muted-foreground mb-1">Last Updated</div>
                  <div className="font-semibold">
                    {data.device_info?.last_updated
                      ? new Date(data.device_info.last_updated * 1000).toLocaleString()
                      : '--'
                    }
                  </div>
                </div>
              </div>

              {/* Alerts & Troubleshooting */}
              {hasAlerts && (
                <div className="space-y-3">
                  <h4 className="font-semibold flex items-center gap-2">
                    <AlertTriangle className="size-4 text-ctp-yellow-900 dark:text-ctp-yellow" />
                    Issues & Solutions
                  </h4>
                  {alerts.map((alert, index) => (
                    <Alert
                      key={index}
                      variant={alert.type === 'error' ? 'destructive' : 'warning'}
                    >
                      {alert.type === 'error' ? <CircleX /> : <AlertTriangle />}
                      <AlertTitle>{alert.message}</AlertTitle>
                      <AlertDescription>
                        <div className="mb-2">Category: {alert.category}</div>

                        {/* Troubleshooting steps based on category */}
                        {alert.category === 'connection_limit' && (
                          <details className="text-sm">
                            <summary className="cursor-pointer font-medium mb-2">
                              <Lightbulb className="inline size-4 mr-1 align-text-bottom" />
                              How to fix this
                            </summary>
                            <ul className="list-disc list-inside space-y-1 ml-2">
                              <li>Close the official Nanit app on your phone/tablet</li>
                              <li>Force-close the app (don&apos;t just minimize it)</li>
                              <li>Wait 30-60 seconds, then try streaming again</li>
                              <li>Make sure no one else is using the Nanit app</li>
                            </ul>
                          </details>
                        )}

                        {alert.category === 'connectivity' && (
                          <details className="text-sm">
                            <summary className="cursor-pointer font-medium mb-2">
                              <Lightbulb className="inline size-4 mr-1 align-text-bottom" />
                              How to fix this
                            </summary>
                            <ul className="list-disc list-inside space-y-1 ml-2">
                              <li>Check camera power connection</li>
                              <li>Verify WiFi connectivity on camera</li>
                              <li>Try power cycling the camera (unplug for 10 seconds)</li>
                              <li>Check the official Nanit app for camera status</li>
                            </ul>
                          </details>
                        )}

                        {alert.category === 'streaming' && (
                          <details className="text-sm">
                            <summary className="cursor-pointer font-medium mb-2">
                              <Lightbulb className="inline size-4 mr-1 align-text-bottom" />
                              How to fix this
                            </summary>
                            <ul className="list-disc list-inside space-y-1 ml-2">
                              <li>Try stopping and restarting the stream</li>
                              <li>Check server resources and disk space</li>
                              <li>Verify RTMP server port (1940) is accessible</li>
                              <li>Camera may need to be restarted</li>
                            </ul>
                          </details>
                        )}

                        {alert.category === 'firmware' && (
                          <details className="text-sm">
                            <summary className="cursor-pointer font-medium mb-2">
                              <Lightbulb className="inline size-4 mr-1 align-text-bottom" />
                              How to fix this
                            </summary>
                            <ul className="list-disc list-inside space-y-1 ml-2">
                              <li>Use the official Nanit app to install the update</li>
                              <li>Ensure camera is connected to WiFi during update</li>
                              <li>Do not power off camera during firmware update</li>
                              <li>Update may take 10-15 minutes to complete</li>
                            </ul>
                          </details>
                        )}
                      </AlertDescription>
                    </Alert>
                  ))}
                </div>
              )}

              {/* Device Details Grid */}
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {/* Device Status */}
                <Card size="sm">
                  <CardHeader className="border-b">
                    <CardTitle className="flex items-center gap-2">
                      <Smartphone className="size-4" />
                      Device Status
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-2 text-sm">
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Hardware:</span>
                      <span className="font-medium">{data.device_info?.hardware_version || '--'}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Mode:</span>
                      <span className="font-medium">{data.device_info?.device_mode || '--'}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Volume:</span>
                      <span className="font-medium">
                        {data.device_info?.volume !== undefined ? `${data.device_info?.volume}%` : '--'}
                      </span>
                    </div>
                  </CardContent>
                </Card>

                {/* Network Status */}
                <Card size="sm">
                  <CardHeader className="border-b">
                    <CardTitle className="flex items-center gap-2">
                      <Wifi className="size-4" />
                      Network
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-2 text-sm">
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">WiFi:</span>
                      <span className="font-medium">{data.device_info?.wifi_network || '--'}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Band:</span>
                      <span className="font-medium">{data.device_info?.wifi_band || '--'}</span>
                    </div>
                  </CardContent>
                </Card>

                {/* Stream Config */}
                <Card size="sm">
                  <CardHeader className="border-b">
                    <CardTitle className="flex items-center gap-2">
                      <Video className="size-4" />
                      Stream Config
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-2 text-sm">
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Mobile:</span>
                      <span className="font-medium">
                        {data.device_info?.mobile_bitrate && data.device_info?.mobile_fps
                          ? `${Math.round(data.device_info?.mobile_bitrate / 1024)}KB/s @ ${data.device_info?.mobile_fps}fps`
                          : '--'}
                      </span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">DVR:</span>
                      <span className="font-medium">
                        {data.device_info?.dvr_bitrate && data.device_info?.dvr_fps
                          ? `${Math.round(data.device_info?.dvr_bitrate / 1024)}KB/s @ ${data.device_info?.dvr_fps}fps`
                          : '--'}
                      </span>
                    </div>
                  </CardContent>
                </Card>
              </div>
            </div>
          ) : null}
        </div>
      )}
    </div>
  )
}
