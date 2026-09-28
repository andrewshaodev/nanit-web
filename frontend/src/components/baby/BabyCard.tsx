import type { Baby } from '@/types/api'
import { Card } from '@/components/ui/card'
import BabyHeader from './BabyHeader'
import VideoSection from './VideoSection'
import SensorGrid from './SensorGrid'
import ControlPanel from './ControlPanel'
import SoundPanel from './SoundPanel'
import HistoricalData from './HistoricalData'

interface BabyCardProps {
  baby: Baby
  collapsed: boolean
  onToggleCollapsed: () => void
  // Left undefined for the camera already at that end
  onMoveUp?: () => void
  onMoveDown?: () => void
}

export default function BabyCard({ baby, collapsed, onToggleCollapsed, onMoveUp, onMoveDown }: BabyCardProps) {
  return (
    // No padding or gap of its own: the gradient header runs edge to edge
    <Card className="gap-0 py-0">
      <BabyHeader
        baby={baby}
        collapsed={collapsed}
        onToggleCollapsed={onToggleCollapsed}
        onMoveUp={onMoveUp}
        onMoveDown={onMoveDown}
      />

      {/* Collapsed cards render nothing below the header, so the video
          player and the charts don't load at all */}
      {!collapsed && (
        <>
          {/* Video beside the readings and controls, rather than above them */}
          <div className="grid gap-4 p-4 lg:grid-cols-3">
            <div className="lg:col-span-2">
              <VideoSection baby={baby} />
            </div>
            <div className="space-y-4">
              <SensorGrid baby={baby} />
              <ControlPanel baby={baby} />
              <SoundPanel baby={baby} />
            </div>
          </div>

          <HistoricalData baby={baby} />
        </>
      )}
    </Card>
  )
}
