import type { Baby } from '@/types/api'
import { Card } from '@/components/ui/card'
import BabyHeader from './BabyHeader'
import VideoSection from './VideoSection'
import SensorGrid from './SensorGrid'
import ControlPanel from './ControlPanel'
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
        <div className="p-6 space-y-8">
          <VideoSection baby={baby} />

          <SensorGrid baby={baby} />

          <ControlPanel baby={baby} />

          <HistoricalData baby={baby} />
        </div>
      )}
    </Card>
  )
}
