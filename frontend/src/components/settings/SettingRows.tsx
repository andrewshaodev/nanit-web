import { Info } from 'lucide-react'

// A settings row: what it is on the left, its actions on the right
export function SettingRow({ title, description, meta, actions }: {
  title: string
  description: React.ReactNode
  meta?: React.ReactNode
  actions: React.ReactNode
}) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
      <div className="min-w-0 space-y-0.5">
        <p className="font-medium">{title}</p>
        <div className="text-muted-foreground">{description}</div>
        {meta && <p className="text-xs text-muted-foreground">{meta}</p>}
      </div>
      <div className="flex shrink-0 gap-2">{actions}</div>
    </div>
  )
}

// A quieter closing row for a note about the box above
export function NoteRow({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex items-start gap-2 bg-muted/40 px-4 py-2 text-xs text-muted-foreground">
      <Info className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
      <p>{children}</p>
    </div>
  )
}
