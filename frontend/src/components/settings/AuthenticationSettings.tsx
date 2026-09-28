import { useState } from 'react';
import { useNavigate } from 'react-router';
import type { AuthStatusResponse } from '@/types/api';
import { api } from '@/lib/api';
import { Info, Loader2 } from 'lucide-react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

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
        <div className="h-4 bg-muted rounded-sm w-1/3 mb-2"></div>
        <div className="h-8 bg-muted rounded-sm w-1/2"></div>
      </div>
    );
  }

  return (
    <>
      <div className="space-y-8">
        {/* Nanit Authentication Section */}
        <div>
          <h3 className="text-lg font-medium mb-4">Nanit Account</h3>
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium">Status</p>
                <div className="flex items-center gap-2 mt-1">
                  <div className={`w-2 h-2 rounded-full ${
                    authStatus.authenticated ? 'bg-ctp-green-900 dark:bg-ctp-green' : 'bg-ctp-red dark:bg-ctp-red'
                  }`} />
                  <p className="text-sm text-muted-foreground">
                    {authStatus.authenticated 
                      ? `Authenticated${authStatus.email ? ` as ${authStatus.email}` : ''}`
                      : authStatus.message
                    }
                  </p>
                </div>
                {authStatus.authenticated && (
                  <div className="mt-2 text-xs text-muted-foreground">
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
              <div className="flex gap-2">
                {authStatus.authenticated ? (
                  <Button variant="destructive" onClick={() => setShowResetConfirmation(true)}>
                    Reset Authentication
                  </Button>
                ) : (
                  <Button onClick={handleReAuthenticate}>
                    Authenticate
                  </Button>
                )}
              </div>
            </div>

            {authStatus.authenticated && (
              <Alert variant="info">
                <Info />
                <AlertDescription>
                  <strong className="text-foreground">Note:</strong> Resetting authentication will stop all monitoring services and require you to re-authenticate with your Nanit account.
                </AlertDescription>
              </Alert>
            )}
          </div>
        </div>

        {/* Web Dashboard Password Protection Section */}
        {webAuthStatus?.password_protection_enabled && (
          <div>
            <h3 className="text-lg font-medium mb-4">Web Dashboard Security</h3>
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium">Password Protection</p>
                  <p className="text-sm text-muted-foreground">
                    {webAuthStatus.password_set 
                      ? 'Password protection is enabled' 
                      : 'No password set'}
                  </p>
                </div>
                <div className="flex gap-2">
                  {!webAuthStatus.password_set ? (
                    <Button onClick={() => openPasswordForm('set')}>
                      Set Password
                    </Button>
                  ) : (
                    <>
                      <Button onClick={() => openPasswordForm('change')}>
                        Change Password
                      </Button>
                      <Button variant="destructive" onClick={() => openPasswordForm('remove')}>
                        Remove Password
                      </Button>
                    </>
                  )}
                </div>
              </div>

              {webAuthStatus.password_set && (
                <Alert variant="info">
                  <Info />
                  <AlertDescription>
                    <p>
                      <strong className="text-foreground">Forgot your password?</strong> You can reset it using the CLI command inside the Docker container:
                      <br />
                      <code className="bg-muted text-foreground px-1 rounded-sm text-xs mt-1 inline-block">
                        docker exec -it YOUR_CONTAINER_NAME /app/bin/nanit --reset-password
                      </code>
                    </p>
                  </AlertDescription>
                </Alert>
              )}
            </div>
          </div>
        )}
      </div>

      {/* Reset Authentication Confirmation Dialog */}
      <AlertDialog
        open={showResetConfirmation}
        onOpenChange={(open) => {
          if (!resetLoading) setShowResetConfirmation(open);
        }}
      >
        <AlertDialogContent className="sm:max-w-md">
          <AlertDialogHeader>
            <AlertDialogTitle>Reset Nanit Authentication</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to reset your Nanit authentication? This will:
            </AlertDialogDescription>
          </AlertDialogHeader>

          <ul className="text-sm text-muted-foreground space-y-1">
            <li>• Stop all monitoring services</li>
            <li>• Clear your authentication session</li>
            <li>• Require you to re-authenticate with your Nanit account</li>
          </ul>

          <AlertDialogFooter>
            <AlertDialogCancel type="button" disabled={resetLoading}>
              Cancel
            </AlertDialogCancel>
            <Button
              variant="destructive"
              onClick={handleResetAuthentication}
              disabled={resetLoading}
            >
              {resetLoading && <Loader2 className="animate-spin" />}
              {resetLoading ? 'Resetting...' : 'Reset Authentication'}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Password Form Dialog */}
      <AlertDialog open={showPasswordForm} onOpenChange={setShowPasswordForm}>
        <AlertDialogContent className="sm:max-w-md">
          <AlertDialogHeader>
            <AlertDialogTitle>
              {formType === 'set' && 'Set Password'}
              {formType === 'change' && 'Change Password'}
              {formType === 'remove' && 'Remove Password'}
            </AlertDialogTitle>
          </AlertDialogHeader>

          <form onSubmit={handlePasswordSubmit} className="grid gap-4">
            {formType === 'set' && (
              <div className="grid gap-2">
                <Label htmlFor="password">
                  New Password
                </Label>
                <Input
                  type="password"
                  id="password"
                  value={formData.password}
                  onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                  required
                  minLength={8}
                  placeholder="Enter a password (minimum 8 characters)"
                />
              </div>
            )}

            {formType === 'change' && (
              <>
                <div className="grid gap-2">
                  <Label htmlFor="currentPassword">
                    Current Password
                  </Label>
                  <Input
                    type="password"
                    id="currentPassword"
                    value={formData.currentPassword}
                    onChange={(e) => setFormData({ ...formData, currentPassword: e.target.value })}
                    required
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="newPassword">
                    New Password
                  </Label>
                  <Input
                    type="password"
                    id="newPassword"
                    value={formData.newPassword}
                    onChange={(e) => setFormData({ ...formData, newPassword: e.target.value })}
                    required
                    minLength={8}
                    placeholder="Enter new password (minimum 8 characters)"
                  />
                </div>
              </>
            )}

            {formType === 'remove' && (
              <div className="grid gap-2">
                <Label htmlFor="currentPassword">
                  Current Password
                </Label>
                <Input
                  type="password"
                  id="currentPassword"
                  value={formData.currentPassword}
                  onChange={(e) => setFormData({ ...formData, currentPassword: e.target.value })}
                  required
                />
                <p className="text-sm text-destructive">
                  This will permanently disable password protection.
                </p>
              </div>
            )}

            <AlertDialogFooter>
              <AlertDialogCancel type="button">
                Cancel
              </AlertDialogCancel>
              <Button
                type="submit"
                variant={formType === 'remove' ? 'destructive' : 'default'}
              >
                {formType === 'set' && 'Set Password'}
                {formType === 'change' && 'Change Password'}
                {formType === 'remove' && 'Remove Password'}
              </Button>
            </AlertDialogFooter>
          </form>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
