import { useState, useEffect, useCallback } from 'react';
import { Camera, CircleAlert, CircleCheck, Radio, ShieldCheck } from 'lucide-react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { api } from '@/lib/api';
import { useStatus } from '@/hooks/useStatus';
import SettingsTabs, { SettingsTab } from '@/components/settings/SettingsTabs';
import DashboardLogin from '@/components/DashboardLogin';
import AuthenticationSettings from '@/components/settings/AuthenticationSettings';
import DeviceSettings from '@/components/settings/DeviceSettings';
import StreamingSettings from '@/components/settings/StreamingSettings';
import type { AuthStatusResponse } from '@/types/api';

interface WebAuthStatus {
  password_protection_enabled: boolean;
  password_set: boolean;
  authenticated: boolean;
}

export default function Settings() {
  const { babies } = useStatus();
  const [authStatus, setAuthStatus] = useState<WebAuthStatus | null>(null);
  const [nanitAuthStatus, setNanitAuthStatus] = useState<AuthStatusResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);
  
  // Login form state
  const [showPasswordLogin, setShowPasswordLogin] = useState(false);
  const [password, setPassword] = useState('');
  const [isLoggingIn, setIsLoggingIn] = useState(false);
  const [loginError, setLoginError] = useState('');

  const loadAuthStatus = useCallback(async () => {
    try {
      const status = await api.getWebAuthStatus();
      setAuthStatus(status);
      
      // Check if authentication is required
      if (status.password_protection_enabled && !status.authenticated) {
        setShowPasswordLogin(true);
      }
    } catch (error) {
      console.error('Failed to load auth status:', error);
      setMessage({ type: 'error', text: 'Failed to load authentication status' });
    } finally {
      setIsLoading(false);
    }
  }, []);

  const loadNanitAuthStatus = useCallback(async () => {
    try {
      const status = await api.getAuthStatus();
      setNanitAuthStatus(status);
    } catch (error) {
      console.error('Failed to load Nanit auth status:', error);
    }
  }, []);

  useEffect(() => {
    // Fetching on mount: both loaders only set state once their request
    // returns, which this rule can't see
    // oxlint-disable-next-line react/set-state-in-effect
    loadAuthStatus();
    loadNanitAuthStatus();
  }, [loadAuthStatus, loadNanitAuthStatus]);

  const handlePasswordLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoggingIn(true);
    setLoginError('');

    try {
      await api.loginWeb(password);
      await loadAuthStatus(); // Refresh auth status
      setShowPasswordLogin(false);
      setPassword('');
    } catch (error: any) {
      setLoginError(error.message || 'Login failed');
    } finally {
      setIsLoggingIn(false);
    }
  };

  // Show password login screen if required
  if (showPasswordLogin) {
    return (
      <DashboardLogin
        purpose="settings"
        password={password}
        onPasswordChange={setPassword}
        error={loginError}
        isLoggingIn={isLoggingIn}
        onSubmit={handlePasswordLogin}
      />
    );
  }

  if (isLoading) {
    return (
      <div className="space-y-4">
        <h1 className="text-xl font-semibold">Settings</h1>
        <div className="animate-pulse space-y-3">
          <div className="h-4 bg-muted rounded-md w-1/4"></div>
          <div className="h-8 bg-muted rounded-md w-1/3"></div>
        </div>
      </div>
    );
  }

  if (!authStatus) {
    return (
      <div className="space-y-4">
        <h1 className="text-xl font-semibold">Settings</h1>
        <Alert variant="destructive">
          <CircleAlert />
          <AlertDescription>Failed to load settings</AlertDescription>
        </Alert>
      </div>
    );
  }

  // Prepare tabs for the settings interface
  const settingsTabs: SettingsTab[] = [
    {
      id: 'authentication',
      label: 'Authentication & Security',
      shortLabel: 'Security',
      icon: ShieldCheck,
      content: (
        <AuthenticationSettings
          authStatus={nanitAuthStatus}
          webAuthStatus={authStatus}
          onAuthStatusUpdate={loadNanitAuthStatus}
          onWebAuthStatusUpdate={loadAuthStatus}
          onMessage={setMessage}
        />
      ),
    },
    {
      id: 'devices',
      label: 'Devices',
      icon: Camera,
      content: (
        <DeviceSettings babies={babies} />
      ),
      disabled: babies.length === 0,
    },
    {
      id: 'streaming',
      label: 'Streaming',
      icon: Radio,
      content: (
        <StreamingSettings babies={babies} />
      ),
      disabled: babies.length === 0,
    },
  ];

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-xl font-semibold">Settings</h1>
        <p className="text-muted-foreground">Manage your Nanit device configuration and preferences</p>
      </div>

      {message && (
        <Alert variant={message.type === 'success' ? 'success' : 'destructive'}>
          {message.type === 'success' ? <CircleCheck /> : <CircleAlert />}
          <AlertDescription>{message.text}</AlertDescription>
        </Alert>
      )}

      <SettingsTabs tabs={settingsTabs} defaultTab="authentication" />
    </div>
  );
}