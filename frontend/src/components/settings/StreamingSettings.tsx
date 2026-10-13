import { useState } from 'react';
import { Check, Copy, House, Radio } from 'lucide-react';
import StreamingLinks from '@/components/baby/StreamingLinks';
import CameraBox from '@/components/CameraBox';
import StatusLabel, { DOT } from '@/components/StatusLabel';
import type { Baby, StreamingInfoResponse } from '@/types/api';
import { copyToClipboard, streamStatus } from '@/lib/utils';
import { maskKey, useKeyRevealed } from '@/lib/keyReveal';
import { Button } from '@/components/ui/button';
import { api } from '@/lib/api';
import { useStreamingInfo } from '@/hooks/useStreamingInfo';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import StreamKey from './StreamKey';
import StreamSource from './StreamSource';

interface StreamingSettingsProps {
  babies: Baby[];
}

const haConfig = (input: string) => `camera:
  - platform: ffmpeg
    name: "Nanit Camera"
    input: "${input}"`;

export default function StreamingSettings({ babies }: StreamingSettingsProps) {
  const { streamingInfo } = useStreamingInfo();
  if (babies.length === 0) {
    return (
      <div className="rounded-md border border-dashed py-8 text-center text-muted-foreground">
        No devices found. Please ensure your Nanit account is authenticated and devices are connected.
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <Alert variant="info">
        <Radio />
        <AlertTitle>Streaming overview</AlertTitle>
        <AlertDescription>
          <ul className="list-disc space-y-0.5 pl-4">
            <li><strong>RTSP:</strong> best for Home Assistant, Frigate, go2rtc and other camera software</li>
            <li><strong>RTMP:</strong> for OBS, VLC and other RTMP players</li>
            <li><strong>HLS:</strong> the dashboard&apos;s fallback, a few seconds behind; it needs a browser signed in to this dashboard</li>
            <li>Start video streaming on the device before using these URLs</li>
          </ul>
        </AlertDescription>
      </Alert>

      {/* A viewer's video comes from the stream server set here */}
      {streamingInfo?.role === 'viewer' && <StreamSource />}

      <StreamKey password={streamingInfo?.password} />

      {babies.map((baby) => {
        const { streaming, label } = streamStatus(baby);
        return (
          <CameraBox
            key={baby.uid}
            baby={baby}
            flush
            status={<StatusLabel dot={streaming ? DOT.good : DOT.idle}>{label}</StatusLabel>}
          >
            <StreamingLinks baby={baby} />
          </CameraBox>
        );
      })}

      <Alert variant="success">
        <House />
        <AlertTitle>Home Assistant integration</AlertTitle>
        <AlertDescription className="space-y-2">
          <p>To add these streams to Home Assistant, use the RTSP URLs in your camera configuration:</p>
          <HAExample config={haConfig(api.getRTSPUrl(babies[0].uid, streamingInfo))} streamingInfo={streamingInfo} />
          <p className="text-xs">
            That&apos;s the first camera&apos;s address; each camera has its own (see its Streaming Links). The
            host comes from NANIT_RTMP_ADDR, and RTSP listens on port 8554 unless NANIT_RTSP_ADDR says otherwise.
          </p>
        </AlertDescription>
      </Alert>
    </div>
  );
}

// The Home Assistant config, with the stream password's key masked unless
// it's shown; Copy gives the real one
function HAExample({ config, streamingInfo }: { config: string; streamingInfo: StreamingInfoResponse | undefined }) {
  const showKey = useKeyRevealed();
  const [copied, setCopied] = useState(false);
  const key = streamingInfo?.password?.enabled ? streamingInfo.password.key : undefined;

  const copy = async () => {
    if (await copyToClipboard(config)) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <div className="relative">
      <pre className="overflow-x-auto rounded-md border bg-muted p-3 pr-12 font-mono text-xs text-foreground">
        {maskKey(config, key, showKey)}
      </pre>
      <Button
        size="icon-sm"
        variant={copied ? 'success' : 'outline'}
        onClick={copy}
        title="Copy config"
        aria-label="Copy config"
        className="absolute right-2 top-2"
      >
        {copied ? <Check /> : <Copy />}
      </Button>
    </div>
  );
}
