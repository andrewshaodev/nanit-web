import { useEffect, useState, type FormEvent } from 'react'
import { CircleAlert, Loader2 } from 'lucide-react'
import AppLogo from '@/components/AppLogo'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent } from '@/components/ui/card'
import OpenIDIcon from '@/components/OpenIDIcon'

interface DashboardLoginProps {
  // What the password unlocks, e.g. "the dashboard"
  purpose: string
  password: string
  onPasswordChange: (password: string) => void
  error?: string
  isLoggingIn: boolean
  onSubmit: (e: FormEvent) => void
  // Whether signing in through the SSO provider is on, and whether there's
  // a password to sign in with as well
  sso?: boolean
  passwordSet?: boolean
}

// Why the last SSO sign-in failed, as the callback reports it
const ssoErrors: Record<string, string> = {
  denied: "The sign-in provider didn't allow it. Check that your account may use this app.",
  unavailable: "Couldn't reach the sign-in provider. Try again later.",
  failed: "Signing in with SSO didn't work. Try again.",
}

// The ?sso_error the callback sent the browser back with, removed from the
// address so a reload doesn't show it again
function useSSOError(): string {
  const [code] = useState(() => new URLSearchParams(window.location.search).get('sso_error') ?? '')
  useEffect(() => {
    if (!code) return
    const url = new URL(window.location.href)
    url.searchParams.delete('sso_error')
    window.history.replaceState(null, '', url)
  }, [code])
  return code ? (ssoErrors[code] ?? ssoErrors.failed) : ''
}

// The password is the way in when the provider is down
const withPassword = (message: string) => message.replace('Try again later.', 'Try again later, or use the password.')

// The web dashboard password screen, shown by any page that requires it
export default function DashboardLogin({
  purpose,
  password,
  onPasswordChange,
  error,
  isLoggingIn,
  onSubmit,
  sso = false,
  passwordSet = true,
}: DashboardLoginProps) {
  const ssoError = useSSOError()
  const showPassword = passwordSet || !sso
  // Back to this page once signed in
  const ssoURL = `/api/webauth/oidc/login?return_to=${encodeURIComponent(window.location.pathname)}`

  return (
    // GitHub's sign-in layout: logo and heading above a narrow, compact box
    <div className="min-h-screen bg-background flex flex-col items-center px-4 pt-16">
      <div className="w-full max-w-sm">
        <div className="mb-4 flex flex-col items-center gap-3 text-center">
          <AppLogo className="size-12" />
          <h1 className="text-2xl font-light">Nanit Web</h1>
          <p className="text-muted-foreground">
            {showPassword && !sso ? `Enter your password to access ${purpose}` : `Sign in to access ${purpose}`}
          </p>
        </div>
        <Card>
          <CardContent className="space-y-4">
            {sso && (
              <Button asChild variant={showPassword ? 'outline' : 'default'} className="w-full">
                <a href={ssoURL}>
                  <OpenIDIcon className="size-4" />
                  Sign in with SSO
                </a>
              </Button>
            )}

            {ssoError && (
              <Alert variant="destructive">
                <CircleAlert />
                <AlertDescription>{showPassword ? withPassword(ssoError) : ssoError}</AlertDescription>
              </Alert>
            )}

            {sso && showPassword && (
              <div className="flex items-center gap-3 text-xs text-muted-foreground" aria-hidden="true">
                <span className="h-px flex-1 bg-border" />
                or
                <span className="h-px flex-1 bg-border" />
              </div>
            )}

            {showPassword && (
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
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
