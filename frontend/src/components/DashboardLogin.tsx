import type { FormEvent } from 'react'
import { Baby, CircleAlert, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent } from '@/components/ui/card'

interface DashboardLoginProps {
  // What the password unlocks, e.g. "the dashboard"
  purpose: string
  password: string
  onPasswordChange: (password: string) => void
  error?: string
  isLoggingIn: boolean
  onSubmit: (e: FormEvent) => void
}

// The web dashboard password screen, shown by any page that requires it
export default function DashboardLogin({ purpose, password, onPasswordChange, error, isLoggingIn, onSubmit }: DashboardLoginProps) {
  return (
    // GitHub's sign-in layout: logo and heading above a narrow, compact box
    <div className="min-h-screen bg-background flex flex-col items-center px-4 pt-16">
      <div className="w-full max-w-sm">
        <div className="mb-4 flex flex-col items-center gap-3 text-center">
          <Baby className="size-10 text-ctp-mauve" aria-hidden="true" />
          <h1 className="text-2xl font-light">Nanit Web</h1>
          <p className="text-muted-foreground">Enter your password to access {purpose}</p>
        </div>
        <Card>
          <CardContent>
            <form className="space-y-4" onSubmit={onSubmit}>
              <div className="space-y-2">
                <Label htmlFor="password" className="sr-only">
                  Password
                </Label>
                <Input
                  id="password"
                  name="password"
                  type="password"
                  autoComplete="current-password"
                  required
                  value={password}
                  onChange={(e) => onPasswordChange(e.target.value)}
                  disabled={isLoggingIn}
                  placeholder="Password"
                />
              </div>

              {error && (
                <Alert variant="destructive">
                  <CircleAlert />
                  <AlertDescription>{error}</AlertDescription>
                </Alert>
              )}

              <Button type="submit" disabled={isLoggingIn} className="w-full">
                {isLoggingIn && <Loader2 className="animate-spin" />}
                {isLoggingIn ? 'Signing in...' : 'Sign in'}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
