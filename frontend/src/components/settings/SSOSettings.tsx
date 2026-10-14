import { useEffect, useState } from 'react';
import useSWR from 'swr';
import { Check, Copy, Eye, EyeOff, Loader2 } from 'lucide-react';
import { api } from '@/lib/api';
import type { SSOSettingsResponse } from '@/types/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import OpenIDIcon from '@/components/OpenIDIcon';
import { NoteRow, SettingRow } from './SettingRows';

interface Fields {
  issuer: string;
  clientId: string;
  clientSecret: string;
}

const fieldsFrom = (settings: SSOSettingsResponse): Fields => ({
  issuer: settings.draft.issuer,
  clientId: settings.draft.client_id,
  clientSecret: '',
});

// What the test sign-in reported, from the address it came back to,
// removed so a reload doesn't show it again
const results: Record<string, { ok: boolean; text: string }> = {
  on: { ok: true, text: 'The test sign-in worked, and sign-in with SSO is on.' },
  denied: {
    ok: false,
    text: "The provider didn't allow the test sign-in. Check that your account is in one of the client's allowed groups.",
  },
  unavailable: { ok: false, text: "Couldn't reach the provider. Check the issuer URL." },
  failed: {
    ok: false,
    text: "The test sign-in didn't work, so nothing changed. Check the client ID, secret and callback URL; the bridge's log says more.",
  },
};

function useTestResult() {
  const [result] = useState(() => {
    const params = new URLSearchParams(window.location.search);
    const code = params.get('sso') ?? params.get('sso_error');
    return code ? (results[code] ?? results.failed) : null;
  });
  useEffect(() => {
    if (!result) return;
    const url = new URL(window.location.href);
    url.searchParams.delete('sso');
    url.searchParams.delete('sso_error');
    window.history.replaceState(null, '', url);
  }, [result]);
  return result;
}

// Signing in to the dashboard through an OpenID Connect provider, such as
// Pocket ID. New settings are tried with a real sign-in before they're
// turned on, so a mistake can't lock anyone out.
export default function SSOSettings({ onChange }: { onChange: () => void }) {
  const { data: settings, mutate } = useSWR<SSOSettingsResponse>('/settings/sso', () => api.getSSOSettings());
  const [fields, setFields] = useState<Fields | null>(null);
  const [showSecret, setShowSecret] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [editing, setEditing] = useState(false);
  const result = useTestResult();

  if (!settings) {
    return <div className="h-32 animate-pulse rounded-md border bg-muted" />;
  }

  const form = fields ?? fieldsFrom(settings);
  const set = (change: Partial<Fields>) => setFields({ ...form, ...change });
  const showForm = !settings.enabled || editing;

  // Saved as the draft, then off to the provider to sign in with it
  const test = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api.saveSSOSettings({
        issuer: form.issuer.trim(),
        client_id: form.clientId.trim(),
        client_secret: form.clientSecret.trim(),
      });
      window.location.assign(api.ssoTestUrl());
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setBusy(false);
    }
  };

  const turnOff = async () => {
    setBusy(true);
    setError(null);
    try {
      await mutate(await api.disableSSO(), { revalidate: false });
      onChange();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const copyCallback = async () => {
    await navigator.clipboard.writeText(settings.callback_url);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="divide-y rounded-md border bg-card">
      <SettingRow
        title={settings.enabled ? 'On' : 'Off'}
        description={
          settings.enabled && settings.active
            ? `People sign in through ${settings.active.issuer}.`
            : 'Sign in through an OpenID Connect provider such as Pocket ID, which decides who may get in.'
        }
        actions={
          // Turning it on goes through the form's test sign-in
          settings.enabled && (
            <>
              {!editing && (
                <Button variant="outline" size="sm" onClick={() => setEditing(true)}>
                  Change
                </Button>
              )}
              <Switch aria-label="Sign in with SSO" checked disabled={busy} onCheckedChange={(on) => !on && turnOff()} />
            </>
          )
        }
      />

      {result && (
        <NoteRow>
          <span className={result.ok ? 'text-ctp-green-900 dark:text-ctp-green' : 'text-destructive'}>{result.text}</span>
        </NoteRow>
      )}

      {showForm && (
        <form onSubmit={test} className="grid gap-4 px-4 py-4">
          <p className="text-muted-foreground">
            In your provider, add an OIDC client with the callback URL below, and choose who may use it (in Pocket ID,
            its allowed user groups). Then enter its details here. <strong>Test and turn on</strong> signs you in with
            them, and only turns SSO on if that works.
          </p>
          <div className="grid gap-2">
            <Label htmlFor="sso-callback">Callback URL</Label>
            <div className="flex gap-2">
              <Input id="sso-callback" value={settings.callback_url} readOnly className="font-mono text-xs" />
              <Button
                type="button"
                size="icon"
                variant="outline"
                onClick={copyCallback}
                title="Copy callback URL"
                aria-label="Copy callback URL"
              >
                {copied ? <Check /> : <Copy />}
              </Button>
            </div>
          </div>
          <div className="grid gap-2">
            <Label htmlFor="sso-issuer">Issuer URL</Label>
            <Input
              id="sso-issuer"
              value={form.issuer}
              onChange={(e) => set({ issuer: e.target.value })}
              placeholder="https://id.example.com"
              autoComplete="off"
              spellCheck={false}
              required
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="sso-client-id">Client ID</Label>
            <Input
              id="sso-client-id"
              value={form.clientId}
              onChange={(e) => set({ clientId: e.target.value })}
              autoComplete="off"
              spellCheck={false}
              required
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="sso-client-secret">Client secret</Label>
            <div className="flex gap-2">
              <Input
                id="sso-client-secret"
                type={showSecret ? 'text' : 'password'}
                value={form.clientSecret}
                onChange={(e) => set({ clientSecret: e.target.value })}
                placeholder={settings.draft_secret_set ? 'Saved; leave empty to keep it' : 'Empty for a public client'}
                autoComplete="off"
                spellCheck={false}
                className={showSecret ? 'font-mono' : undefined}
              />
              <Button
                type="button"
                size="icon"
                variant="outline"
                onClick={() => setShowSecret(!showSecret)}
                title={showSecret ? 'Hide client secret' : 'Show client secret'}
                aria-label={showSecret ? 'Hide client secret' : 'Show client secret'}
              >
                {showSecret ? <EyeOff /> : <Eye />}
              </Button>
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button type="submit" size="sm" disabled={busy}>
              {busy ? <Loader2 className="animate-spin" /> : <OpenIDIcon className="size-4" />}
              {settings.enabled ? 'Test and save' : 'Test and turn on'}
            </Button>
            {editing && (
              <Button
                type="button"
                size="sm"
                variant="ghost"
                onClick={() => {
                  setEditing(false);
                  setFields(null);
                }}
                disabled={busy}
              >
                Cancel
              </Button>
            )}
          </div>
        </form>
      )}

      {error && <NoteRow>Couldn&apos;t save: {error}</NoteRow>}

      {settings.enabled && (
        <SettingRow
          title="Sign out"
          description="Ends this browser's dashboard session. Signing out of the provider doesn't."
          actions={
            <Button
              variant="outline"
              size="sm"
              onClick={async () => {
                await api.logoutWeb();
                window.location.assign('/');
              }}
            >
              Sign out
            </Button>
          }
        />
      )}
    </div>
  );
}
