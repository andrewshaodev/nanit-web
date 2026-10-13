import { House, Radio } from 'lucide-react';
import StreamingLinks from '@/components/baby/StreamingLinks';
import CameraBox from '@/components/CameraBox';
import StatusLabel, { DOT } from '@/components/StatusLabel';
import type { Baby } from '@/types/api';
import { streamStatus } from '@/lib/utils';
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
            <li><strong>HLS:</strong> best for web browsers and mobile apps</li>
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
          <pre className="overflow-x-auto rounded-md border bg-muted p-3 font-mono text-xs text-foreground">
            {haConfig(api.getRTSPUrl(babies[0].uid, streamingInfo))}
          </pre>
          <p className="text-xs">
            That&apos;s the first camera&apos;s address; each camera has its own (see its Streaming Links). The
            host comes from NANIT_RTMP_ADDR, and RTSP listens on port 8554 unless NANIT_RTSP_ADDR says otherwise.
          </p>
        </AlertDescription>
      </Alert>
    </div>
  );
}
