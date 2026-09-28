import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import useSWR from 'swr'
import MainLayout from '@/components/layout/MainLayout'
import BabyCard from '@/components/baby/BabyCard'
import DashboardLogin from '@/components/DashboardLogin'
import { Card, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import ErrorMessage from '@/components/ui/ErrorMessage'
import { api } from '@/lib/api'
import { useStatus } from '@/hooks/useStatus'
import { useCameraLayout } from '@/hooks/useCameraLayout'
import type { AuthStatusResponse, WebAuthStatusResponse } from '@/types/api'

export default function Dashboard() {
  const navigate = useNavigate()
  const [showPasswordLogin, setShowPasswordLogin] = useState(false)
  const [password, setPassword] = useState('')
  const [loginError, setLoginError] = useState('')
  const [isLoggingIn, setIsLoggingIn] = useState(false)
  
  // Check web authentication status first
  const { data: webAuthStatus, mutate: mutateWebAuth } = useSWR<WebAuthStatusResponse>(
    '/webauth/status',
    () => api.getWebAuthStatus(),
    {
      revalidateOnFocus: false,
      revalidateOnReconnect: false,
    }
  )

  // Check Nanit authentication status (only if web auth passes)
  const { data: authStatus } = useSWR<AuthStatusResponse>(
    webAuthStatus?.authenticated ? '/auth/status' : null,
    () => api.getAuthStatus(),
    {
      revalidateOnFocus: false,
      revalidateOnReconnect: false,
    }
  )

  // Only fetch baby data if both authentications pass
  const { babies, isLoading, isError } = useStatus(
    webAuthStatus?.authenticated && authStatus?.authenticated
  )
  const { sortBabies, isCollapsed, toggleCollapsed, move } = useCameraLayout()

  useEffect(() => {
    if (webAuthStatus) {
      // If password protection is enabled but user not authenticated, show login
      if (webAuthStatus.password_protection_enabled && !webAuthStatus.authenticated) {
        setShowPasswordLogin(true)
        return
      }
      
      // If web auth passes but Nanit auth fails, redirect to setup
      if (webAuthStatus.authenticated && authStatus && !authStatus.authenticated) {
        navigate('/setup')
      }
    }
  }, [webAuthStatus, authStatus, navigate])

  const handlePasswordLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsLoggingIn(true)
    setLoginError('')

    try {
      await api.loginWeb(password)
      await mutateWebAuth() // Refresh auth status
      setShowPasswordLogin(false)
      setPassword('')
    } catch (error: any) {
      setLoginError(error.message || 'Login failed')
    } finally {
      setIsLoggingIn(false)
    }
  }

  // Show password login screen if required
  if (showPasswordLogin) {
    return (
      <DashboardLogin
        purpose="the dashboard"
        password={password}
        onPasswordChange={setPassword}
        error={loginError}
        isLoggingIn={isLoggingIn}
        onSubmit={handlePasswordLogin}
      />
    )
  }

  if (isLoading) {
    return (
      <MainLayout>
        <div className="flex items-center justify-center min-h-64">
          <LoadingSpinner size="lg" />
        </div>
      </MainLayout>
    )
  }

  if (isError) {
    return (
      <MainLayout>
        <ErrorMessage 
          title="Connection Error"
          message="Unable to connect to the Nanit bridge. Please check your connection."
        />
      </MainLayout>
    )
  }

  if (babies.length === 0) {
    return (
      <MainLayout>
        <div className="text-center py-12">
          <Card className="max-w-md mx-auto">
            <CardHeader>
              <CardTitle className="text-xl">No babies configured</CardTitle>
              <CardDescription>
                Make sure you have authenticated and configured your Nanit account.
              </CardDescription>
            </CardHeader>
          </Card>
        </div>
      </MainLayout>
    )
  }

  const sortedBabies = sortBabies(babies)

  return (
    <MainLayout>
      <div className="space-y-8">
        {sortedBabies.map((baby, index) => (
          <BabyCard
            key={baby.uid}
            baby={baby}
            collapsed={isCollapsed(baby.uid)}
            onToggleCollapsed={() => toggleCollapsed(baby.uid)}
            onMoveUp={index > 0 ? () => move(sortedBabies, baby.uid, -1) : undefined}
            onMoveDown={index < sortedBabies.length - 1 ? () => move(sortedBabies, baby.uid, 1) : undefined}
          />
        ))}
      </div>
    </MainLayout>
  )
}