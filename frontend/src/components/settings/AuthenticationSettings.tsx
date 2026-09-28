import { useState } from 'react';
import { useNavigate } from 'react-router';
import type { AuthStatusResponse } from '@/types/api';
import { api } from '@/lib/api';

interface WebAuthStatus {
  password_protection_enabled: boolean;
  password_set: boolean;
  authenticated: boolean;
}

interface AuthenticationSettingsProps {
  authStatus: AuthStatusResponse | null;
  webAuthStatus: WebAuthStatus | null;
  onAuthStatusUpdate: () => void;
  onWebAuthStatusUpdate: () => void;
  onMessage: (message: { type: 'success' | 'error'; text: string }) => void;
}

export default function AuthenticationSettings({ 
  authStatus, 
  webAuthStatus,
  onAuthStatusUpdate, 
  onWebAuthStatusUpdate,
  onMessage 
}: AuthenticationSettingsProps) {
  const navigate = useNavigate();
  const [showResetConfirmation, setShowResetConfirmation] = useState(false);
  const [resetLoading, setResetLoading] = useState(false);
  
  // Password form state
  const [showPasswordForm, setShowPasswordForm] = useState(false);
  const [formType, setFormType] = useState<'set' | 'change' | 'remove'>('set');
  const [formData, setFormData] = useState({
    password: '',
    currentPassword: '',
    newPassword: ''
  });

  const handleResetAuthentication = async () => {
    setResetLoading(true);
    onMessage({ type: 'success', text: '' }); // Clear previous messages

    try {
      const result = await api.resetAuth();
      
      if (result.success) {
        onMessage({ type: 'success', text: result.message });
        await onAuthStatusUpdate();
        setShowResetConfirmation(false);
      }
    } catch (error: any) {
      onMessage({ 
        type: 'error', 
        text: error.message || 'Failed to reset authentication' 
      });
    } finally {
      setResetLoading(false);
    }
  };

  const handleReAuthenticate = () => {
    navigate('/setup');
  };

  const handlePasswordSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    onMessage({ type: 'success', text: '' }); // Clear previous messages

    try {
      let result;
      
      if (formType === 'set') {
        result = await api.setWebPassword(formData.password);
      } else if (formType === 'change') {
        result = await api.changeWebPassword(formData.currentPassword, formData.newPassword);
      } else if (formType === 'remove') {
        result = await api.removeWebPassword(formData.currentPassword);
      }

      if (result) {
        onMessage({ type: 'success', text: result.message });
        setShowPasswordForm(false);
        setFormData({ password: '', currentPassword: '', newPassword: '' });
        await onWebAuthStatusUpdate();
      }
    } catch (error: any) {
      onMessage({ 
        type: 'error', 
        text: error.message || 'An error occurred' 
      });
    }
  };

  const openPasswordForm = (type: 'set' | 'change' | 'remove') => {
    setFormType(type);
    setShowPasswordForm(true);
    onMessage({ type: 'success', text: '' }); // Clear messages
    setFormData({ password: '', currentPassword: '', newPassword: '' });
  };

  if (!authStatus) {
    return (
      <div className="animate-pulse">
        <div className="h-4 bg-ctp-surface0 rounded-sm w-1/3 mb-2"></div>
        <div className="h-8 bg-ctp-surface0 rounded-sm w-1/2"></div>
      </div>
    );
  }

  return (
    <>
      <div className="space-y-8">
        {/* Nanit Authentication Section */}
        <div>
          <h3 className="text-lg font-medium text-ctp-text mb-4">Nanit Account</h3>
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-ctp-text">Status</p>
                <div className="flex items-center gap-2 mt-1">
                  <div className={`w-2 h-2 rounded-full ${
                    authStatus.authenticated ? 'bg-ctp-green-900 dark:bg-ctp-green' : 'bg-ctp-red dark:bg-ctp-red'
                  }`} />
                  <p className="text-sm text-ctp-subtext1">
                    {authStatus.authenticated 
                      ? `Authenticated${authStatus.email ? ` as ${authStatus.email}` : ''}`
                      : authStatus.message
                    }
                  </p>
                </div>
                {authStatus.authenticated && (
                  <div className="mt-2 text-xs text-ctp-subtext1">
                    {authStatus.babies_count && (
                      <span>{authStatus.babies_count} device{authStatus.babies_count !== 1 ? 's' : ''} • </span>
                    )}
                    Services: {authStatus.services_running ? 'Running' : 'Stopped'}
                    {authStatus.auth_time && (
                      <span> • Authenticated: {new Date(authStatus.auth_time * 1000).toLocaleDateString()}</span>
                    )}
                  </div>
                )}
              </div>
              <div className="flex space-x-2">
                {authStatus.authenticated ? (
                  <button
                    onClick={() => setShowResetConfirmation(true)}
                    className="px-4 py-2 bg-ctp-red dark:bg-ctp-red text-ctp-base rounded-md hover:bg-ctp-red-600 dark:hover:bg-ctp-red/85 focus:outline-hidden focus:ring-2 focus:ring-ctp-red"
                  >
                    Reset Authentication
                  </button>
                ) : (
                  <button
                    onClick={handleReAuthenticate}
                    className="px-4 py-2 bg-ctp-blue-700 dark:bg-ctp-blue text-ctp-base rounded-md hover:bg-ctp-blue-800 dark:hover:bg-ctp-blue/85 focus:outline-hidden focus:ring-2 focus:ring-ctp-blue"
                  >
                    Authenticate
                  </button>
                )}
              </div>
            </div>

            {authStatus.authenticated && (
              <div className="bg-ctp-blue/10 border border-ctp-blue/30 rounded-md p-4">
                <p className="text-ctp-blue-700 dark:text-ctp-blue text-sm">
                  <strong>Note:</strong> Resetting authentication will stop all monitoring services and require you to re-authenticate with your Nanit account.
                </p>
              </div>
            )}
          </div>
        </div>

        {/* Web Dashboard Password Protection Section */}
        {webAuthStatus?.password_protection_enabled && (
          <div>
            <h3 className="text-lg font-medium text-ctp-text mb-4">Web Dashboard Security</h3>
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium text-ctp-text">Password Protection</p>
                  <p className="text-sm text-ctp-subtext1">
                    {webAuthStatus.password_set 
                      ? 'Password protection is enabled' 
                      : 'No password set'}
                  </p>
                </div>
                <div className="flex space-x-2">
                  {!webAuthStatus.password_set ? (
                    <button
                      onClick={() => openPasswordForm('set')}
                      className="px-4 py-2 bg-ctp-blue-700 dark:bg-ctp-blue text-ctp-base rounded-md hover:bg-ctp-blue-800 dark:hover:bg-ctp-blue/85 focus:outline-hidden focus:ring-2 focus:ring-ctp-blue"
                    >
                      Set Password
                    </button>
                  ) : (
                    <>
                      <button
                        onClick={() => openPasswordForm('change')}
                        className="px-4 py-2 bg-ctp-blue-700 dark:bg-ctp-blue text-ctp-base rounded-md hover:bg-ctp-blue-800 dark:hover:bg-ctp-blue/85 focus:outline-hidden focus:ring-2 focus:ring-ctp-blue"
                      >
                        Change Password
                      </button>
                      <button
                        onClick={() => openPasswordForm('remove')}
                        className="px-4 py-2 bg-ctp-red dark:bg-ctp-red text-ctp-base rounded-md hover:bg-ctp-red-600 dark:hover:bg-ctp-red/85 focus:outline-hidden focus:ring-2 focus:ring-ctp-red"
                      >
                        Remove Password
                      </button>
                    </>
                  )}
                </div>
              </div>

              {webAuthStatus.password_set && (
                <div className="bg-ctp-blue/10 border border-ctp-blue/30 rounded-md p-4">
                  <p className="text-ctp-blue-700 dark:text-ctp-blue text-sm">
                    <strong>Forgot your password?</strong> You can reset it using the CLI command inside the Docker container:
                    <br />
                    <code className="bg-ctp-blue/20 px-1 rounded-sm text-xs mt-1 inline-block">
                      docker exec -it YOUR_CONTAINER_NAME /app/bin/nanit --reset-password
                    </code>
                  </p>
                </div>
              )}
            </div>
          </div>
        )}
      </div>

      {/* Reset Authentication Confirmation Modal */}
      {showResetConfirmation && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-ctp-base rounded-lg p-6 w-full max-w-md mx-4">
            <h3 className="text-lg font-medium text-ctp-text mb-4">
              Reset Nanit Authentication
            </h3>
            
            <p className="text-sm text-ctp-subtext1 mb-6">
              Are you sure you want to reset your Nanit authentication? This will:
            </p>
            
            <ul className="text-sm text-ctp-subtext1 mb-6 space-y-1">
              <li>• Stop all monitoring services</li>
              <li>• Clear your authentication session</li>
              <li>• Require you to re-authenticate with your Nanit account</li>
            </ul>

            <div className="flex justify-end space-x-3">
              <button
                type="button"
                onClick={() => setShowResetConfirmation(false)}
                disabled={resetLoading}
                className="px-4 py-2 text-ctp-subtext1 bg-ctp-surface0 rounded-md hover:bg-ctp-surface1 focus:outline-hidden focus:ring-2 focus:ring-ctp-overlay0 disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                onClick={handleResetAuthentication}
                disabled={resetLoading}
                className="px-4 py-2 bg-ctp-red dark:bg-ctp-red text-ctp-base rounded-md hover:bg-ctp-red-600 dark:hover:bg-ctp-red/85 focus:outline-hidden focus:ring-2 focus:ring-ctp-red disabled:opacity-50"
              >
                {resetLoading ? 'Resetting...' : 'Reset Authentication'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Password Form Modal */}
      {showPasswordForm && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-ctp-base rounded-lg p-6 w-full max-w-md mx-4">
            <h3 className="text-lg font-medium text-ctp-text mb-4">
              {formType === 'set' && 'Set Password'}
              {formType === 'change' && 'Change Password'}
              {formType === 'remove' && 'Remove Password'}
            </h3>

            <form onSubmit={handlePasswordSubmit}>
              {formType === 'set' && (
                <div className="mb-4">
                  <label htmlFor="password" className="block text-sm font-medium text-ctp-subtext1 mb-2">
                    New Password
                  </label>
                  <input
                    type="password"
                    id="password"
                    value={formData.password}
                    onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                    className="w-full px-3 py-2 border border-ctp-surface1 rounded-md focus:outline-hidden focus:ring-2 focus:ring-ctp-blue"
                    required
                    minLength={8}
                    placeholder="Enter a password (minimum 8 characters)"
                  />
                </div>
              )}

              {formType === 'change' && (
                <>
                  <div className="mb-4">
                    <label htmlFor="currentPassword" className="block text-sm font-medium text-ctp-subtext1 mb-2">
                      Current Password
                    </label>
                    <input
                      type="password"
                      id="currentPassword"
                      value={formData.currentPassword}
                      onChange={(e) => setFormData({ ...formData, currentPassword: e.target.value })}
                      className="w-full px-3 py-2 border border-ctp-surface1 rounded-md focus:outline-hidden focus:ring-2 focus:ring-ctp-blue"
                      required
                    />
                  </div>
                  <div className="mb-4">
                    <label htmlFor="newPassword" className="block text-sm font-medium text-ctp-subtext1 mb-2">
                      New Password
                    </label>
                    <input
                      type="password"
                      id="newPassword"
                      value={formData.newPassword}
                      onChange={(e) => setFormData({ ...formData, newPassword: e.target.value })}
                      className="w-full px-3 py-2 border border-ctp-surface1 rounded-md focus:outline-hidden focus:ring-2 focus:ring-ctp-blue"
                      required
                      minLength={8}
                      placeholder="Enter new password (minimum 8 characters)"
                    />
                  </div>
                </>
              )}

              {formType === 'remove' && (
                <div className="mb-4">
                  <label htmlFor="currentPassword" className="block text-sm font-medium text-ctp-subtext1 mb-2">
                    Current Password
                  </label>
                  <input
                    type="password"
                    id="currentPassword"
                    value={formData.currentPassword}
                    onChange={(e) => setFormData({ ...formData, currentPassword: e.target.value })}
                    className="w-full px-3 py-2 border border-ctp-surface1 rounded-md focus:outline-hidden focus:ring-2 focus:ring-ctp-blue"
                    required
                  />
                  <p className="text-sm text-ctp-red dark:text-ctp-red mt-2">
                    This will permanently disable password protection.
                  </p>
                </div>
              )}

              <div className="flex justify-end space-x-3">
                <button
                  type="button"
                  onClick={() => setShowPasswordForm(false)}
                  className="px-4 py-2 text-ctp-subtext1 bg-ctp-surface0 rounded-md hover:bg-ctp-surface1 focus:outline-hidden focus:ring-2 focus:ring-ctp-overlay0"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className={`px-4 py-2 text-ctp-base rounded-md focus:outline-hidden focus:ring-2 ${
                    formType === 'remove'
                      ? 'bg-ctp-red dark:bg-ctp-red hover:bg-ctp-red-600 dark:hover:bg-ctp-red/85 focus:ring-ctp-red'
                      : 'bg-ctp-blue-700 dark:bg-ctp-blue hover:bg-ctp-blue-800 dark:hover:bg-ctp-blue/85 focus:ring-ctp-blue'
                  }`}
                >
                  {formType === 'set' && 'Set Password'}
                  {formType === 'change' && 'Change Password'}
                  {formType === 'remove' && 'Remove Password'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </>
  );
}