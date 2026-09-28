import { useState } from 'react'
import { api } from '@/lib/api'
import type { Baby } from '@/types/api'
import { Loader2, TriangleAlert } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Alert, AlertDescription } from '@/components/ui/alert'

interface ControlPanelProps {
  baby: Baby
}

interface ControlButtonProps {
  onClick: () => Promise<void>
  disabled: boolean
  loading: boolean
  children: React.ReactNode
  variant?: 'primary' | 'secondary' | 'danger'
}

function ControlButton({ onClick, disabled, loading, children, variant = 'primary' }: ControlButtonProps) {
  const [feedback, setFeedback] = useState<'success' | 'error' | null>(null)

  const handleClick = async () => {
    try {
      await onClick()
      setFeedback('success')
      setTimeout(() => setFeedback(null), 2000)
    } catch (error) {
      console.error('Control action failed:', error)
      setFeedback('error')
      setTimeout(() => setFeedback(null), 3000)
    }
  }

  const buttonVariant = () => {
    if (feedback === 'success') return 'success'
    if (feedback === 'error') return 'destructive'

    switch (variant) {
      case 'secondary':
        return 'secondary'
      case 'danger':
        return 'destructive'
      default:
        // Neutral, as GitHub's everyday buttons are; solid mauve is kept for
        // a page's main action
        return 'outline'
    }
  }

  const getButtonContent = () => {
    if (loading) {
      return (
        <>
          <Loader2 className="animate-spin" />
          Sending...
        </>
      )
    }
    
    if (feedback === 'success') return 'Sent!'
    if (feedback === 'error') return 'Failed'
    
    return children
  }

  return (
    <Button
      onClick={handleClick}
      disabled={disabled || loading || !!feedback}
      variant={buttonVariant()}
      className="w-full"
    >
      {getButtonContent()}
    </Button>
  )
}

export default function ControlPanel({ baby }: ControlPanelProps) {
  const [loadingStates, setLoadingStates] = useState<Record<string, boolean>>({})

  const setLoading = (action: string, loading: boolean) => {
    setLoadingStates(prev => ({ ...prev, [action]: loading }))
  }

  const handleNightLightToggle = async () => {
    setLoading('nightlight', true)
    try {
      await api.toggleNightLight(baby.uid)
      // Optionally refresh status here
    } finally {
      setLoading('nightlight', false)
    }
  }

  const handleStandbyToggle = async () => {
    setLoading('standby', true)
    try {
      await api.toggleStandby(baby.uid)
      // Optionally refresh status here
    } finally {
      setLoading('standby', false)
    }
  }

  const isDisabled = !baby.websocket_alive

  return (
    <div className="space-y-2">
      <h3 className="font-semibold">Controls</h3>
      
      {!baby.websocket_alive && (
        <Alert variant="warning">
          <TriangleAlert />
          <AlertDescription>
            Device is offline. Controls are disabled until connection is restored.
          </AlertDescription>
        </Alert>
      )}
      
      <div className="grid grid-cols-2 gap-2">
        <ControlButton
          onClick={handleNightLightToggle}
          disabled={isDisabled}
          loading={loadingStates.nightlight || false}
        >
          {baby.night_light ? 'Turn Off Night Light' : 'Turn On Night Light'}
        </ControlButton>
        
        <ControlButton
          onClick={handleStandbyToggle}
          disabled={isDisabled}
          loading={loadingStates.standby || false}
        >
          {baby.standby ? 'Exit Standby' : 'Enter Standby'}
        </ControlButton>
      </div>
      
      <div className="text-xs text-muted-foreground">
        Control commands are sent to the device and may take a few seconds to take effect.
      </div>
    </div>
  )
}