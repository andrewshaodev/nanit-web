import { House, Radio } from 'lucide-react';
import StreamingLinks from '@/components/baby/StreamingLinks';
import CameraBox from '@/components/CameraBox';
import StatusLabel, { DOT } from '@/components/StatusLabel';
import type { Baby } from '@/types/api';
import { streamStatus } from '@/lib/utils';
import { api } from '@/lib/api';
import { useStreamingInfo } from '@/hooks/useStreamingInfo';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';

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
            <li><strong>RTMP:</strong> best for Home Assistant, OBS, VLC and other video software</li>
            <li><strong>HLS:</strong> best for web browsers and mobile apps</li>
            <li>Start video streaming on the device before using these URLs</li>
          </ul>
        </AlertDescription>
      </Alert>

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
          <p>To add these streams to Home Assistant, use the RTMP URLs in your camera configuration:</p>
          <pre className="overflow-x-auto rounded-md border bg-muted p-3 font-mono text-xs text-foreground">
            {haConfig(api.getRTMPUrl(babies[0].uid, streamingInfo))}
          </pre>
          <p className="text-xs">
            That&apos;s the first camera&apos;s address; each camera has its own (see its Streaming Links). The
            address is NANIT_RTMP_ADDR, and it has to be reachable from both the camera and Home Assistant.
          </p>
        </AlertDescription>
      </Alert>
    </div>
  );
}
