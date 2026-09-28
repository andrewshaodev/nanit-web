import { useState } from 'react';
import { useNavigate } from 'react-router';
import type { AuthStatusResponse } from '@/types/api';
import { api } from '@/lib/api';
import { Info, Loader2 } from 'lucide-react';
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

// A settings row: what it is on the left, its actions on the right
function SettingRow({ title, description, meta, actions }: {
  title: string
  description: React.ReactNode
  meta?: React.ReactNode
  actions: React.ReactNode
}) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
      <div className="min-w-0 space-y-0.5">
        <p className="font-medium">{title}</p>
        <div className="text-muted-foreground">{description}</div>
        {meta && <p className="text-xs text-muted-foreground">{meta}</p>}
      </div>
      <div className="flex shrink-0 gap-2">{actions}</div>
    </div>
  )
}

// A quieter closing row for a note about the box above
function NoteRow({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex items-start gap-2 bg-muted/40 px-4 py-2 text-xs text-muted-foreground">
      <Info className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
      <p>{children}</p>
    </div>
  )
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
      <div className="space-y-6">
        {/* Nanit account: a GitHub-style box of rows */}
        <section className="space-y-2">
          <h3 className="font-semibold">Nanit account</h3>
          <div className="divide-y rounded-md border bg-card">
            <SettingRow
              title="Status"
              description={
                <span className="flex items-center gap-2">
                  <span
                    className={`size-2 rounded-full ${authStatus.authenticated ? 'bg-ctp-green-900 dark:bg-ctp-green' : 'bg-ctp-red'}`}
                    aria-hidden="true"
                  />
                  {authStatus.authenticated
                    ? `Authenticated${authStatus.email ? ` as ${authStatus.email}` : ''}`
                    : authStatus.message}
                </span>
              }
              meta={
                authStatus.authenticated && (
                  <>
                    {authStatus.babies_count ? `${authStatus.babies_count} device${authStatus.babies_count !== 1 ? 's' : ''} · ` : ''}
                    Services {authStatus.services_running ? 'running' : 'stopped'}
                    {authStatus.auth_time ? ` · Signed in ${new Date(authStatus.auth_time * 1000).toLocaleDateString()}` : ''}
                  </>
                )
              }
              actions={
                authStatus.authenticated ? (
                  <Button variant="destructive" size="sm" onClick={() => setShowResetConfirmation(true)}>
                    Reset Authentication
                  </Button>
                ) : (
                  <Button size="sm" onClick={handleReAuthenticate}>
                    Authenticate
                  </Button>
                )
              }
            />
            {authStatus.authenticated && (
              <NoteRow>
                Resetting authentication stops all monitoring services until you sign in to Nanit again.
              </NoteRow>
            )}
          </div>
        </section>

        {/* Web dashboard password */}
        {webAuthStatus?.password_protection_enabled && (
          <section className="space-y-2">
            <h3 className="font-semibold">Web dashboard security</h3>
            <div className="divide-y rounded-md border bg-card">
              <SettingRow
                title="Password protection"
                description={webAuthStatus.password_set ? 'Password protection is enabled' : 'No password set'}
                actions={
                  !webAuthStatus.password_set ? (
                    <Button size="sm" onClick={() => openPasswordForm('set')}>
                      Set Password
                    </Button>
                  ) : (
                    <>
                      <Button variant="outline" size="sm" onClick={() => openPasswordForm('change')}>
                        Change Password
                      </Button>
                      <Button variant="destructive" size="sm" onClick={() => openPasswordForm('remove')}>
                        Remove Password
                      </Button>
                    </>
                  )
                }
              />
              {webAuthStatus.password_set && (
                <NoteRow>
                  Forgot it? Reset it from inside the Docker container:{' '}
                  <code className="rounded-sm bg-muted px-1 py-0.5 font-mono text-xs text-foreground">
                    docker exec -it YOUR_CONTAINER_NAME /app/bin/nanit --reset-password
                  </code>
                </NoteRow>
              )}
            </div>
          </section>
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
