import { useState } from 'react';
import StreamingLinks from '@/components/baby/StreamingLinks';
import type { Baby } from '@/types/api';
import { displayName } from '@/lib/utils';

interface StreamingSettingsProps {
  babies: Baby[];
}

export default function StreamingSettings({ babies }: StreamingSettingsProps) {
  const [activeTab, setActiveTab] = useState(0);

  if (babies.length === 0) {
    return (
      <div className="text-center py-8 text-ctp-subtext1">
        No devices found. Please ensure your Nanit account is authenticated and devices are connected.
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Global Streaming Information */}
      <div className="bg-ctp-blue/10 border border-ctp-blue/30 rounded-lg p-4">
        <h4 className="font-medium text-ctp-blue-700 dark:text-ctp-blue mb-2">📡 Streaming Overview</h4>
        <div className="text-sm text-ctp-blue-700 dark:text-ctp-blue space-y-1">
          <p>• <strong>RTMP:</strong> Best for Home Assistant, OBS, VLC, and other video software</p>
          <p>• <strong>HLS:</strong> Best for web browsers and modern mobile applications</p>
          <p>• Start video streaming on the device before using these URLs</p>
          <p>• Streaming quality depends on your network connection and device settings</p>
        </div>
      </div>

      {/* Device Tabs */}
      {babies.length > 1 && (
        <div className="border-b border-ctp-surface0">
          <nav className="-mb-px flex space-x-8">
            {babies.map((baby, index) => (
              <button
                key={baby.uid}
                onClick={() => setActiveTab(index)}
                className={`py-2 px-1 border-b-2 font-medium text-sm ${
                  activeTab === index
                    ? 'border-ctp-blue text-ctp-blue-700 dark:text-ctp-blue'
                    : 'border-transparent text-ctp-subtext1 hover:text-ctp-subtext1 hover:border-ctp-surface1'
                }`}
              >
                {displayName(baby)}
                {baby.stream_state === 'streaming' && (
                  <span className="ml-2 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-ctp-green/20 text-ctp-green-900 dark:text-ctp-green">
                    Live
                  </span>
                )}
              </button>
            ))}
          </nav>
        </div>
      )}

      {/* Streaming Links Content */}
      <div className="space-y-4">
        {babies.length === 1 ? (
          <div>
            <div className="flex items-center justify-between mb-4">
              <h4 className="text-lg font-medium text-ctp-text">{babies[0].name}</h4>
              <div className="flex items-center gap-2">
                <div className={`w-2 h-2 rounded-full ${
                  babies[0].stream_state === 'streaming' ? 'bg-ctp-green-900 dark:bg-ctp-green' : 'bg-ctp-overlay0'
                }`} />
                <span className="text-sm text-ctp-subtext1">
                  {babies[0].stream_state === 'streaming' ? 'Streaming' : 'Not streaming'}
                </span>
              </div>
            </div>
            <StreamingLinks baby={babies[0]} />
          </div>
        ) : (
          <div>
            <div className="flex items-center justify-between mb-4">
              <h4 className="text-lg font-medium text-ctp-text">{babies[activeTab]?.name}</h4>
              <div className="flex items-center gap-2">
                <div className={`w-2 h-2 rounded-full ${
                  babies[activeTab]?.stream_state === 'streaming' ? 'bg-ctp-green-900 dark:bg-ctp-green' : 'bg-ctp-overlay0'
                }`} />
                <span className="text-sm text-ctp-subtext1">
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
        <div className="mt-8 bg-ctp-mantle rounded-lg p-4">
          <h4 className="text-sm font-medium text-ctp-text mb-3">Streaming Status</h4>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {babies.map((baby) => (
              <div key={baby.uid} className="bg-ctp-base rounded-lg p-3 border border-ctp-surface0">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="font-medium text-ctp-text">{displayName(baby)}</p>
                    <p className="text-xs text-ctp-subtext1">
                      {baby.websocket_alive ? 'Connected' : 'Disconnected'}
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    <div className={`w-2 h-2 rounded-full ${
                      baby.stream_state === 'streaming' ? 'bg-ctp-green-900 dark:bg-ctp-green' : 'bg-ctp-overlay0'
                    }`} />
                    <span className="text-xs text-ctp-subtext1">
                      {baby.stream_state === 'streaming' ? 'Live' : 'Offline'}
                    </span>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Home Assistant Integration Help */}
      <div className="bg-ctp-green/10 border border-ctp-green/30 rounded-lg p-4">
        <h4 className="font-medium text-ctp-green-900 dark:text-ctp-green mb-2">🏠 Home Assistant Integration</h4>
        <div className="text-sm text-ctp-green-900 dark:text-ctp-green space-y-2">
          <p>To add these streams to Home Assistant, use the RTMP URLs in your camera configuration:</p>
          <div className="bg-ctp-green/20 p-3 rounded-sm font-mono text-xs overflow-x-auto">
            <div>camera:</div>
            <div>&nbsp;&nbsp;- platform: ffmpeg</div>
            <div>&nbsp;&nbsp;&nbsp;&nbsp;name: &quot;Nanit Camera&quot;</div>
            <div>&nbsp;&nbsp;&nbsp;&nbsp;input: &quot;rtmp://YOUR_SERVER_IP:1935/camera_uid&quot;</div>
          </div>
          <p className="text-xs text-ctp-green-900 dark:text-ctp-green">
            Replace YOUR_SERVER_IP with the IP address of this server and camera_uid with your device&apos;s UID.
          </p>
        </div>
      </div>
    </div>
  );
}