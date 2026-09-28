import type { FormEvent } from 'react'
import { CircleAlert, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

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
    <div className="min-h-screen bg-background flex items-center justify-center py-12 px-4">
      <Card className="max-w-md w-full">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl font-bold">Nanit Dashboard</CardTitle>
          <CardDescription>Enter your password to access {purpose}</CardDescription>
        </CardHeader>
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
  )
}
