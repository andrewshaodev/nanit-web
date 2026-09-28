import { useState } from 'react'
import { useNavigate } from 'react-router'
import { api } from '@/lib/api'
import { CircleAlert, Loader2, MessageSquareText } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export default function SetupPage() {
  const navigate = useNavigate()
  const [step, setStep] = useState<'login' | '2fa'>('login')
  const [formData, setFormData] = useState({
    email: '',
    password: '',
    mfaCode: '',
  })
  const [mfaToken, setMfaToken] = useState<any>(null)
  const [mfaDelivery, setMfaDelivery] = useState<{ channel?: string; phoneSuffix?: string }>({})
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState('')

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsLoading(true)
    setError('')

    try {
      const response = await api.login(formData.email, formData.password)
      
      if (response.success && response.mfa_token) {
        setMfaToken(response.mfa_token)
        setMfaDelivery({ channel: response.channel, phoneSuffix: response.phone_suffix })
        setStep('2fa')
      } else {
        setError(response.error || 'Login failed')
      }
    } catch (error) {
      setError('Failed to connect to server')
    } finally {
      setIsLoading(false)
    }
  }

  const handleVerify2FA = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsLoading(true)
    setError('')

    try {
      const response = await api.verify2FA(
        formData.email,
        formData.password,
        mfaToken,
        formData.mfaCode,
        mfaDelivery.channel
      )
      
      if (response.success) {
        // Redirect to dashboard
        navigate('/')
      } else {
        setError(response.error || 'Verification failed')
      }
    } catch (error) {
      setError('Failed to verify code')
    } finally {
      setIsLoading(false)
    }
  }

  const handleInputChange = (field: string, value: string) => {
    setFormData(prev => ({ ...prev, [field]: value }))
    setError('')
  }

  const errorAlert = error && (
    <Alert variant="destructive">
      <CircleAlert />
      <AlertDescription>{error}</AlertDescription>
    </Alert>
  )

  return (
    <div className="min-h-screen bg-background flex items-center justify-center px-4">
      <div className="max-w-md w-full">
        <div className="text-center mb-8">
          <h1 className="text-3xl font-bold text-gradient mb-2">
            Nanit Dashboard
          </h1>
          <p className="text-muted-foreground">
            Set up your Nanit Home Assistant Bridge
          </p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="text-xl">
              {step === 'login' ? 'Sign In to Nanit' : 'Two-Factor Authentication'}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {step === 'login' ? (
              <form onSubmit={handleLogin} className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="setup-email">Email</Label>
                  <Input
                    id="setup-email"
                    type="email"
                    autoComplete="username"
                    value={formData.email}
                    onChange={(e) => handleInputChange('email', e.target.value)}
                    placeholder="your@email.com"
                    required
                  />
                </div>

                <div className="space-y-2">
                  <Label htmlFor="setup-password">Password</Label>
                  <Input
                    id="setup-password"
                    type="password"
                    autoComplete="current-password"
                    value={formData.password}
                    onChange={(e) => handleInputChange('password', e.target.value)}
                    placeholder="Your password"
                    required
                  />
                </div>

                {errorAlert}

                <Button type="submit" disabled={isLoading} className="w-full">
                  {isLoading && <Loader2 className="animate-spin" />}
                  {isLoading ? 'Signing In...' : 'Sign In'}
                </Button>
              </form>
            ) : (
              <div className="space-y-6">
                <Alert variant="info">
                  <MessageSquareText />
                  <AlertDescription>
                    {mfaDelivery.channel === 'sms'
                      ? `Nanit texted a verification code to the phone ending in ${mfaDelivery.phoneSuffix ?? '??'}. Enter it below.`
                      : 'Check your email for a verification code from Nanit and enter it below.'}
                  </AlertDescription>
                </Alert>

                <form onSubmit={handleVerify2FA} className="space-y-4">
                  <div className="space-y-2">
                    <Label htmlFor="setup-mfa-code">Verification Code</Label>
                    <Input
                      id="setup-mfa-code"
                      type="text"
                      inputMode="numeric"
                      autoComplete="one-time-code"
                      value={formData.mfaCode}
                      onChange={(e) => handleInputChange('mfaCode', e.target.value)}
                      placeholder="Enter 6-digit code"
                      maxLength={6}
                      required
                    />
                  </div>

                  {errorAlert}

                  <div className="flex gap-3">
                    <Button
                      type="button"
                      variant="secondary"
                      className="flex-1"
                      onClick={() => {
                        setStep('login')
                        setMfaToken(null)
                        setFormData(prev => ({ ...prev, mfaCode: '' }))
                      }}
                    >
                      Back
                    </Button>

                    <Button type="submit" disabled={isLoading} className="flex-1">
                      {isLoading && <Loader2 className="animate-spin" />}
                      {isLoading ? 'Verifying...' : 'Verify'}
                    </Button>
                  </div>
                </form>
              </div>
            )}
          </CardContent>
        </Card>

        <div className="text-center mt-6 text-sm text-muted-foreground">
          <p>This will securely store your authentication for the Nanit bridge.</p>
        </div>
      </div>
    </div>
  )
}
