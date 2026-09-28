import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router';
import { api } from '@/lib/api';
import { useStatus } from '@/hooks/useStatus';
import SettingsTabs, { SettingsTab } from '@/components/settings/SettingsTabs';
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
  // Debug: Add console log to verify component is loading
  console.log('Settings component rendering');
  
  const navigate = useNavigate();
  const { babies, isLoading: statusLoading, isError: statusError } = useStatus();
  const [authStatus, setAuthStatus] = useState<WebAuthStatus | null>(null);
  const [nanitAuthStatus, setNanitAuthStatus] = useState<AuthStatusResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);
  
  // Login form state
  const [showPasswordLogin, setShowPasswordLogin] = useState(false);
  const [password, setPassword] = useState('');
  const [isLoggingIn, setIsLoggingIn] = useState(false);
  const [loginError, setLoginError] = useState('');

  useEffect(() => {
    loadAuthStatus();
    loadNanitAuthStatus();
  }, []);

  const loadAuthStatus = async () => {
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
  };

  const loadNanitAuthStatus = async () => {
    try {
      const status = await api.getAuthStatus();
      setNanitAuthStatus(status);
    } catch (error) {
      console.error('Failed to load Nanit auth status:', error);
    }
  };

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
      <div className="min-h-screen bg-ctp-mantle flex items-center justify-center py-12 px-4 sm:px-6 lg:px-8">
        <div className="max-w-md w-full space-y-8">
          <div>
            <h2 className="mt-6 text-center text-3xl font-extrabold text-ctp-text">
              Nanit Dashboard
            </h2>
            <p className="mt-2 text-center text-sm text-ctp-subtext1">
              Enter your password to access settings
            </p>
          </div>
          <form className="mt-8 space-y-6" onSubmit={handlePasswordLogin}>
            <div>
              <label htmlFor="password" className="sr-only">
                Password
              </label>
              <input
                id="password"
                name="password"
                type="password"
                required
                className="appearance-none rounded-md relative block w-full px-3 py-2 border border-ctp-surface1 placeholder-ctp-overlay1 text-ctp-text focus:outline-hidden focus:ring-ctp-lavender focus:border-ctp-lavender focus:z-10 sm:text-sm"
                placeholder="Password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                disabled={isLoggingIn}
              />
            </div>

            {loginError && (
              <div className="text-ctp-red dark:text-ctp-red text-sm text-center">
                {loginError}
              </div>
            )}

            <div>
              <button
                type="submit"
                disabled={isLoggingIn}
                className="group relative w-full flex justify-center py-2 px-4 border border-transparent text-sm font-medium rounded-md text-ctp-base bg-ctp-lavender-900 dark:bg-ctp-lavender hover:bg-ctp-lavender-950 dark:hover:bg-ctp-lavender/85 focus:outline-hidden focus:ring-2 focus:ring-offset-2 focus:ring-ctp-lavender disabled:opacity-50"
              >
                {isLoggingIn ? 'Signing in...' : 'Sign in'}
              </button>
            </div>
          </form>
        </div>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="p-6">
        <div className="bg-ctp-base rounded-lg shadow-sm p-6">
          <h2 className="text-2xl font-bold text-ctp-text mb-4">Settings</h2>
          <div className="animate-pulse">
            <div className="h-4 bg-ctp-surface0 rounded-sm w-1/4 mb-4"></div>
            <div className="h-8 bg-ctp-surface0 rounded-sm w-1/3"></div>
          </div>
        </div>
      </div>
    );
  }

  if (!authStatus) {
    return (
      <div className="p-6">
        <div className="bg-ctp-base rounded-lg shadow-sm p-6">
          <h2 className="text-2xl font-bold text-ctp-text mb-4">Settings</h2>
          <div className="text-ctp-red dark:text-ctp-red">Failed to load settings</div>
        </div>
      </div>
    );
  }

  // Prepare tabs for the settings interface
  const settingsTabs: SettingsTab[] = [
    {
      id: 'authentication',
      label: 'Authentication & Security',
      icon: '🔐',
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
      icon: '📱',
      content: (
        <DeviceSettings babies={babies} />
      ),
      disabled: babies.length === 0,
    },
    {
      id: 'streaming',
      label: 'Streaming',
      icon: '📡',
      content: (
        <StreamingSettings babies={babies} />
      ),
      disabled: babies.length === 0,
    },
  ];

  return (
    <div className="p-6">
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-ctp-text">Settings</h2>
        <p className="text-ctp-subtext1 mt-1">Manage your Nanit device configuration and preferences</p>
      </div>
      
      {message && (
        <div className={`mb-6 p-4 rounded-md ${
          message.type === 'success' 
            ? 'bg-ctp-green/10 text-ctp-green-900 dark:text-ctp-green border border-ctp-green/30' 
            : 'bg-ctp-red/10 text-ctp-red dark:text-ctp-red border border-ctp-red/30'
        }`}>
          {message.text}
        </div>
      )}

      <SettingsTabs tabs={settingsTabs} defaultTab="authentication" />
    </div>
  );
}