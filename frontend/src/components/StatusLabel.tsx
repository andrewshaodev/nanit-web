import type { ReactNode } from 'react'

interface StatusLabelProps {
  // Classes for the status dot, e.g. 'bg-ctp-green-900 dark:bg-ctp-green'
  dot: string
  children: ReactNode
  className?: string
}

// A GitHub-style label: a small outlined pill with a status dot
export default function StatusLabel({ dot, children, className = '' }: StatusLabelProps) {
  return (
    <span className={`inline-flex items-center gap-1.5 rounded-full border bg-background px-2 py-0.5 text-xs whitespace-nowrap ${className}`}>
      <span className={`size-2 rounded-full ${dot}`} aria-hidden="true" />
      {children}
    </span>
  )
}

export const DOT = {
  good: 'bg-ctp-green-900 dark:bg-ctp-green',
  bad: 'bg-ctp-red',
  idle: 'bg-ctp-overlay0',
}
