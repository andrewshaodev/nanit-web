import { AlertTriangle } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'

interface ErrorMessageProps {
  title?: string
  message: string
  action?: {
    label: string
    onClick: () => void
  }
}

export default function ErrorMessage({ title, message, action }: ErrorMessageProps) {
  return (
    <Alert variant="destructive" className="max-w-md mx-auto p-4 border-l-4 border-l-destructive">
      <AlertTriangle />
      {title && (
        <AlertTitle>
          {title}
        </AlertTitle>
      )}
      <AlertDescription>
        <p>
          {message}
        </p>
        {action && (
          <Button
            variant="destructive"
            size="sm"
            onClick={action.onClick}
            className="mt-3"
          >
            {action.label}
          </Button>
        )}
      </AlertDescription>
    </Alert>
  )
}
