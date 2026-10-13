import { ChevronDown, ChevronRight } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface CollapseButtonProps {
  collapsed: boolean
  onToggle: () => void
  // What it collapses, for screen readers: "crying log"
  label: string
}

// The chevron at the start of a card section's header
export default function CollapseButton({ collapsed, onToggle, label }: CollapseButtonProps) {
  return (
    <Button
      variant="ghost"
      size="icon-sm"
      // shadcn's ghost button highlights an open disclosure; these don't
      className="aria-expanded:bg-transparent aria-expanded:hover:bg-accent"
      onClick={onToggle}
      aria-expanded={!collapsed}
      aria-label={`${collapsed ? 'Expand' : 'Collapse'} ${label}`}
      title={collapsed ? 'Expand' : 'Collapse'}
    >
      {collapsed ? <ChevronRight /> : <ChevronDown />}
    </Button>
  )
}
