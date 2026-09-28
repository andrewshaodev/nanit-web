import { useState } from 'react'
import { useNavigate } from 'react-router'
import { api } from '@/lib/api'
import LoadingSpinner from '@/components/ui/LoadingSpinner'

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

  return (
    <div className="min-h-screen bg-ctp-mantle flex items-center justify-center px-4">
      <div className="max-w-md w-full">
        <div className="text-center mb-8">
          <h1 className="text-3xl font-bold text-gradient mb-2">
            Nanit Dashboard
          </h1>
          <p className="text-ctp-subtext1">
            Set up your Nanit Home Assistant Bridge
          </p>
        </div>

        <div className="card p-6">
          {step === 'login' ? (
            <>
              <h2 className="text-xl font-semibold text-ctp-text mb-6">
                Sign In to Nanit
              </h2>
              
              <form onSubmit={handleLogin} className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-ctp-subtext1 mb-2">
                    Email
                  </label>
                  <input
                    type="email"
                    value={formData.email}
                    onChange={(e) => handleInputChange('email', e.target.value)}
                    className="w-full px-3 py-2 border border-ctp-surface1 rounded-md focus:outline-hidden focus:ring-2 focus:ring-ctp-blue"
                    placeholder="your@email.com"
                    required
                  />
                </div>

                <div>
                  <label className="block text-sm font-medium text-ctp-subtext1 mb-2">
                    Password
                  </label>
                  <input
                    type="password"
                    value={formData.password}
                    onChange={(e) => handleInputChange('password', e.target.value)}
                    className="w-full px-3 py-2 border border-ctp-surface1 rounded-md focus:outline-hidden focus:ring-2 focus:ring-ctp-blue"
                    placeholder="Your password"
                    required
                  />
                </div>

                {error && (
                  <div className="bg-ctp-red/10 border-l-4 border-ctp-red p-3 rounded-sm">
                    <div className="text-sm text-ctp-red dark:text-ctp-red">{error}</div>
                  </div>
                )}

                <button
                  type="submit"
                  disabled={isLoading}
                  className="w-full btn btn-primary disabled:opacity-50"
                >
                  {isLoading ? (
                    <div className="flex items-center justify-center gap-2">
                      <LoadingSpinner size="sm" />
                      <span>Signing In...</span>
                    </div>
                  ) : (
                    'Sign In'
                  )}
                </button>
              </form>
            </>
          ) : (
            <>
              <h2 className="text-xl font-semibold text-ctp-text mb-6">
                Two-Factor Authentication
              </h2>
              
              <div className="bg-ctp-blue/10 border-l-4 border-ctp-blue p-3 rounded-sm mb-6">
                <div className="text-sm text-ctp-blue-700 dark:text-ctp-blue">
                  {mfaDelivery.channel === 'sms'
                    ? `Nanit texted a verification code to the phone ending in ${mfaDelivery.phoneSuffix ?? '??'}. Enter it below.`
                    : 'Check your email for a verification code from Nanit and enter it below.'}
                </div>
              </div>

              <form onSubmit={handleVerify2FA} className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-ctp-subtext1 mb-2">
                    Verification Code
                  </label>
                  <input
                    type="text"
                    value={formData.mfaCode}
                    onChange={(e) => handleInputChange('mfaCode', e.target.value)}
                    className="w-full px-3 py-2 border border-ctp-surface1 rounded-md focus:outline-hidden focus:ring-2 focus:ring-ctp-blue"
                    placeholder="Enter 6-digit code"
                    maxLength={6}
                    required
                  />
                </div>

                {error && (
                  <div className="bg-ctp-red/10 border-l-4 border-ctp-red p-3 rounded-sm">
                    <div className="text-sm text-ctp-red dark:text-ctp-red">{error}</div>
                  </div>
                )}

                <div className="flex gap-3">
                  <button
                    type="button"
                    onClick={() => {
                      setStep('login')
                      setMfaToken(null)
                      setFormData(prev => ({ ...prev, mfaCode: '' }))
                    }}
                    className="flex-1 btn btn-secondary"
                  >
                    Back
                  </button>
                  
                  <button
                    type="submit"
                    disabled={isLoading}
                    className="flex-1 btn btn-primary disabled:opacity-50"
                  >
                    {isLoading ? (
                      <div className="flex items-center justify-center gap-2">
                        <LoadingSpinner size="sm" />
                        <span>Verifying...</span>
                      </div>
                    ) : (
                      'Verify'
                    )}
                  </button>
                </div>
              </form>
            </>
          )}
        </div>

        <div className="text-center mt-6 text-sm text-ctp-subtext1">
          <p>This will securely store your authentication for the Nanit bridge.</p>
        </div>
      </div>
    </div>
  )
}