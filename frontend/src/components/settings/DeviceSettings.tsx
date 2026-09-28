import DeviceInfo from '@/components/baby/DeviceInfo';
import CameraBox from '@/components/CameraBox';
import StatusLabel, { DOT } from '@/components/StatusLabel';
import type { Baby } from '@/types/api';

interface DeviceSettingsProps {
  babies: Baby[];
}

// One box per camera, with its details inside
export default function DeviceSettings({ babies }: DeviceSettingsProps) {
  if (babies.length === 0) {
    return (
      <div className="rounded-md border border-dashed py-8 text-center text-muted-foreground">
        No devices found. Please ensure your Nanit account is authenticated and devices are connected.
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {babies.map((baby) => (
        <CameraBox
          key={baby.uid}
          baby={baby}
          flush
          status={
            <StatusLabel dot={baby.websocket_alive ? DOT.good : DOT.bad}>
              {baby.websocket_alive ? 'Online' : 'Offline'}
            </StatusLabel>
          }
        >
          <DeviceInfo baby={baby} />
        </CameraBox>
      ))}
    </div>
  );
}
