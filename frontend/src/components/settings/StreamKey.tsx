import { useState } from 'react';
import useSWR, { useSWRConfig } from 'swr';
import { Check, Copy, Eye, EyeOff, KeyRound, Loader2, RefreshCw } from 'lucide-react';
import { api } from '@/lib/api';
import { copyToClipboard } from '@/lib/utils';
import type { StreamingInfoResponse, StreamPasswordInfo, WebAuthStatusResponse } from '@/types/api';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { NoteRow, SettingRow } from './SettingRows';

interface StreamKeyProps {
  password: StreamPasswordInfo | undefined;
}

// The password on the RTSP and RTMP outputs. Its key is generated here, and
// the streaming links include it while it's on.
export default function StreamKey({ password }: StreamKeyProps) {
  const { mutate } = useSWRConfig();
  const { data: webAuth } = useSWR<WebAuthStatusResponse>('/webauth/status', () => api.getWebAuthStatus(), {
    revalidateOnFocus: false,
  });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [revealed, setRevealed] = useState(false);
  const [copied, setCopied] = useState(false);
  const [confirmRegenerate, setConfirmRegenerate] = useState(false);

  const apply = async (change: () => Promise<StreamPasswordInfo>) => {
    setBusy(true);
    setError(null);
    try {
      const next = await change();
      // The links on this page carry the key, so they follow it
      await mutate<StreamingInfoResponse>(
        '/streaming/info',
        (info) => (info ? { ...info, password: next } : info),
        { revalidate: true },
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const copy = async () => {
    if (password?.key && (await copyToClipboard(password.key))) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const enabled = password?.enabled ?? false;

  return (
    <section className="space-y-2">
      <h3 className="flex items-center gap-2 font-semibold">
        <KeyRound className="size-4" />
        Stream password
      </h3>
      <div className="divide-y rounded-md border bg-card">
        <SettingRow
          title="Require a password"
          description={
            enabled
              ? 'On: RTSP and RTMP players need the key below. The links on this page include it.'
              : 'Off: anything that can reach the RTSP and RTMP ports can watch.'
          }
          actions={
            <span className="flex items-center gap-2">
              {busy && <Loader2 className="size-4 animate-spin text-muted-foreground" />}
              <Switch
                aria-label="Require a password for RTSP and RTMP"
                checked={enabled}
                disabled={busy || !password}
                onCheckedChange={(on) => {
                  // The first key is shown straight away, ready to copy
                  if (on && !password?.key) setRevealed(true);
                  void apply(() => api.setStreamPassword(on));
                }}
              />
            </span>
          }
        />

        {enabled && password?.key && (
          <SettingRow
            title="Key"
            description={
              <span className="block space-y-1">
                <code className="block break-all font-mono text-sm text-foreground">
                  {revealed ? password.key : '•'.repeat(password.key.length)}
                </code>
                <span className="block text-xs">
                  RTSP: user <code className="font-mono text-foreground">{password.user}</code>, the key as the
                  password. RTMP: <code className="font-mono text-foreground">?key=</code> on the address.
                </span>
              </span>
            }
            actions={
              <>
                <Button
                  size="icon-sm"
                  variant="outline"
                  onClick={() => setRevealed(!revealed)}
                  title={revealed ? 'Hide key' : 'Show key'}
                  aria-label={revealed ? 'Hide key' : 'Show key'}
                >
                  {revealed ? <EyeOff /> : <Eye />}
                </Button>
                <Button
                  size="icon-sm"
                  variant={copied ? 'success' : 'outline'}
                  onClick={copy}
                  title="Copy key"
                  aria-label="Copy key"
                >
                  {copied ? <Check /> : <Copy />}
                </Button>
                <Button size="sm" variant="outline" disabled={busy} onClick={() => setConfirmRegenerate(true)}>
                  <RefreshCw />
                  New key
                </Button>
              </>
            }
          />
        )}

        {error && <NoteRow>Couldn&apos;t change the stream password: {error}</NoteRow>}
        {enabled && webAuth && !webAuth.password_set && (
          <NoteRow>
            The dashboard has no password, so anyone who can open it can see this key. Set one under Authentication.
          </NoteRow>
        )}
        <NoteRow>
          Changing it disconnects players that are watching, and they need the new settings to reconnect. The
          dashboard&apos;s own video and the camera&apos;s stream to this bridge aren&apos;t affected. Viewer instances
          enter the key under Settings → Streaming → Stream server.
        </NoteRow>
      </div>

      <AlertDialog open={confirmRegenerate} onOpenChange={setConfirmRegenerate}>
        <AlertDialogContent className="sm:max-w-md">
          <AlertDialogHeader>
            <AlertDialogTitle>Make a new key?</AlertDialogTitle>
            <AlertDialogDescription>
              Players using the current key, such as Home Assistant or a viewer instance, are disconnected and
              can&apos;t reconnect until they have the new one.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <Button
              variant="destructive"
              onClick={() => {
                setConfirmRegenerate(false);
                setRevealed(true);
                void apply(() => api.regenerateStreamPassword());
              }}
            >
              Make a new key
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </section>
  );
}
