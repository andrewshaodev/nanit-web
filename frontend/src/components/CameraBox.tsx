import type { ReactNode } from 'react'
import { Cctv } from 'lucide-react'
import type { Baby } from '@/types/api'
import { displayName } from '@/lib/utils'
import { Card } from '@/components/ui/card'

interface CameraBoxProps {
  baby: Baby
  // Right-hand side of the header row, e.g. a status label
  status?: ReactNode
  // Content that lays itself out edge to edge, such as disclosure rows
  flush?: boolean
  children: ReactNode
}

// One camera as a GitHub-style box: a mauve-tinted header row with its name
// and uid, then its content. Matches the dashboard's camera boxes.
export default function CameraBox({ baby, status, flush = false, children }: CameraBoxProps) {
  return (
    <Card className="gap-0 py-0">
      <div className="flex items-center gap-2 border-b border-ctp-mauve/25 bg-ctp-mauve/10 px-4 py-2">
        <Cctv className="size-4 shrink-0 text-ctp-mauve" aria-hidden="true" />
        <span className="font-semibold">{displayName(baby)}</span>
        <code className="font-mono text-xs text-muted-foreground">{baby.uid}</code>
        <span className="ml-auto">{status}</span>
      </div>
      <div className={flush ? '' : 'p-4'}>{children}</div>
    </Card>
  )
}
