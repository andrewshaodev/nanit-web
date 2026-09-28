import { useState } from 'react';
import { House, Radio } from 'lucide-react';
import StreamingLinks from '@/components/baby/StreamingLinks';
import type { Baby } from '@/types/api';
import { displayName } from '@/lib/utils';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';

interface StreamingSettingsProps {
  babies: Baby[];
}

export default function StreamingSettings({ babies }: StreamingSettingsProps) {
  const [activeTab, setActiveTab] = useState(0);

  if (babies.length === 0) {
    return (
      <div className="text-center py-8 text-muted-foreground">
        No devices found. Please ensure your Nanit account is authenticated and devices are connected.
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Global Streaming Information */}
      <Alert variant="info">
        <Radio />
        <AlertTitle>Streaming Overview</AlertTitle>
        <AlertDescription className="[&_p:not(:last-child)]:mb-1">
          <p>• <strong>RTMP:</strong> Best for Home Assistant, OBS, VLC, and other video software</p>
          <p>• <strong>HLS:</strong> Best for web browsers and modern mobile applications</p>
          <p>• Start video streaming on the device before using these URLs</p>
          <p>• Streaming quality depends on your network connection and device settings</p>
        </AlertDescription>
      </Alert>

      {/* Device Tabs */}
      {babies.length > 1 && (
        <Tabs value={String(activeTab)} onValueChange={(value) => setActiveTab(Number(value))}>
          <TabsList variant="line" className="overflow-x-auto">
            {babies.map((baby, index) => (
              <TabsTrigger key={baby.uid} value={String(index)}>
                {displayName(baby)}
                {baby.stream_state === 'streaming' && (
                  <Badge className="ml-1 bg-ctp-green/20 text-ctp-green-900 dark:text-ctp-green">
                    Live
                  </Badge>
                )}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      )}

      {/* Streaming Links Content */}
      <div className="space-y-4">
        {babies.length === 1 ? (
          <div>
            <div className="flex items-center justify-between mb-4">
              <h4 className="text-lg font-medium">{babies[0].name}</h4>
              <div className="flex items-center gap-2">
                <div className={`w-2 h-2 rounded-full ${
                  babies[0].stream_state === 'streaming' ? 'bg-ctp-green-900 dark:bg-ctp-green' : 'bg-ctp-overlay0'
                }`} />
                <span className="text-sm text-muted-foreground">
                  {babies[0].stream_state === 'streaming' ? 'Streaming' : 'Not streaming'}
                </span>
              </div>
            </div>
            <StreamingLinks baby={babies[0]} />
          </div>
        ) : (
          <div>
            <div className="flex items-center justify-between mb-4">
              <h4 className="text-lg font-medium">{babies[activeTab]?.name}</h4>
              <div className="flex items-center gap-2">
                <div className={`w-2 h-2 rounded-full ${
                  babies[activeTab]?.stream_state === 'streaming' ? 'bg-ctp-green-900 dark:bg-ctp-green' : 'bg-ctp-overlay0'
                }`} />
                <span className="text-sm text-muted-foreground">
                  {babies[activeTab]?.stream_state === 'streaming' ? 'Streaming' : 'Not streaming'}
                </span>
              </div>
            </div>
            <StreamingLinks baby={babies[activeTab]} />
          </div>
        )}
      </div>

      {/* Streaming Status Summary for Multiple Devices */}
      {babies.length > 1 && (
        <Card size="sm" className="mt-8 bg-muted/50">
          <CardHeader>
            <CardTitle>Streaming Status</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {babies.map((baby) => (
                <div key={baby.uid} className="bg-card rounded-lg p-3 border">
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="font-medium">{displayName(baby)}</p>
                      <p className="text-xs text-muted-foreground">
                        {baby.websocket_alive ? 'Connected' : 'Disconnected'}
                      </p>
                    </div>
                    <div className="flex items-center gap-2">
                      <div className={`w-2 h-2 rounded-full ${
                        baby.stream_state === 'streaming' ? 'bg-ctp-green-900 dark:bg-ctp-green' : 'bg-ctp-overlay0'
                      }`} />
                      <span className="text-xs text-muted-foreground">
                        {baby.stream_state === 'streaming' ? 'Live' : 'Offline'}
                      </span>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Home Assistant Integration Help */}
      <Alert variant="success">
        <House />
        <AlertTitle>Home Assistant Integration</AlertTitle>
        <AlertDescription className="space-y-2">
          <p>To add these streams to Home Assistant, use the RTMP URLs in your camera configuration:</p>
          <div className="bg-ctp-green/15 text-foreground p-3 rounded-sm font-mono text-xs overflow-x-auto">
            <div>camera:</div>
            <div>&nbsp;&nbsp;- platform: ffmpeg</div>
            <div>&nbsp;&nbsp;&nbsp;&nbsp;name: &quot;Nanit Camera&quot;</div>
            <div>&nbsp;&nbsp;&nbsp;&nbsp;input: &quot;rtmp://YOUR_SERVER_IP:1935/camera_uid&quot;</div>
          </div>
          <p className="text-xs">
            Replace YOUR_SERVER_IP with the IP address of this server and camera_uid with your device&apos;s UID.
          </p>
        </AlertDescription>
      </Alert>
    </div>
  );
}
