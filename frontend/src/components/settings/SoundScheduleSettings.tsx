import useSWR from 'swr'
import { api } from '@/lib/api'
import type { Baby, SoundStatus } from '@/types/api'
import { TriangleAlert } from 'lucide-react'
import CameraBox from '@/components/CameraBox'
import { Alert, AlertDescription } from '@/components/ui/alert'
import SoundSchedules, { useSoundSchedules } from './SoundSchedules'

// One camera's schedules. Its sounds are read from the camera while it's
// online, sharing the dashboard's Sound panel's cache.
function CameraSchedules({ baby }: { baby: Baby }) {
  const { data } = useSWR<SoundStatus>(baby.websocket_alive ? `/sound/${baby.uid}` : null, () => api.getSound(baby.uid), {
    revalidateOnFocus: false,
  })
  return (
    <CameraBox baby={baby}>
      <SoundSchedules babyUid={baby.uid} tracks={data?.tracks ?? []} />
    </CameraBox>
  )
}

// This device's offset from UTC, in minutes, as the bridge gives its own
const BROWSER_UTC_OFFSET = -new Date().getTimezoneOffset()

// Sounds the bridge plays between set times of day, per camera
export default function SoundScheduleSettings({ babies }: { babies: Baby[] }) {
  // The bridge's clock and role are the same for every camera
  const { data } = useSoundSchedules(babies[0]?.uid ?? '')

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">
        Play one of the camera’s sounds between set times of day, such as white noise overnight. The bridge runs
        them, so nothing needs to be open. A sound stopped by hand stays stopped until the next start.
      </p>
      {data && (
        <p className="text-sm text-muted-foreground">
          Times are on the bridge’s clock ({data.time_zone}).
          {BROWSER_UTC_OFFSET !== data.utc_offset_minutes && ' That’s not this device’s time zone.'}
        </p>
      )}
      {data && !data.active && (
        <Alert variant="warning">
          <TriangleAlert />
          <AlertDescription>
            This bridge is a viewer, so it doesn’t run schedules. Set them on the stream server’s dashboard.
          </AlertDescription>
        </Alert>
      )}
      {babies.map((baby) => (
        <CameraSchedules key={baby.uid} baby={baby} />
      ))}
    </div>
  )
}
