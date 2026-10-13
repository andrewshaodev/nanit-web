import { useState } from 'react';
import useSWR, { useSWRConfig } from 'swr';
import { Eye, EyeOff, Loader2, Server } from 'lucide-react';
import { api } from '@/lib/api';
import { displayName } from '@/lib/utils';
import type { StreamSourceResponse } from '@/types/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import StatusLabel, { DOT } from '@/components/StatusLabel';
import { NoteRow } from './SettingRows';

interface Fields {
  host: string;
  port: string;
  key: string;
}

const fieldsFrom = (source: StreamSourceResponse): Fields => ({
  host: source.host,
  port: String(source.port || 8554),
  key: source.key ?? '',
});

// A viewer's stream server: where its video comes from. Saving reconnects
// straight away, and each camera's line says whether video is arriving.
export default function StreamSource() {
  const { mutate } = useSWRConfig();
  const { data: source, mutate: mutateSource } = useSWR<StreamSourceResponse>(
    '/streaming/source',
    () => api.getStreamSource(),
    // The cameras' statuses change as they connect
    { refreshInterval: 3000, revalidateOnFocus: true },
  );

  // The form starts from the saved settings, and keeps what's typed until
  // it's saved
  const [fields, setFields] = useState<Fields | null>(null);
  const [showKey, setShowKey] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!source) {
    return (
      <section className="space-y-2">
        <h3 className="flex items-center gap-2 font-semibold">
          <Server className="size-4" />
          Stream server
        </h3>
        <div className="h-32 animate-pulse rounded-md border bg-muted" />
      </section>
    );
  }

  const form = fields ?? fieldsFrom(source);
  const saved = fieldsFrom(source);
  const dirty = form.host !== saved.host || form.port !== saved.port || form.key !== saved.key;
  const set = (change: Partial<Fields>) => setFields({ ...form, ...change });

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      const next = await api.setStreamSource({
        host: form.host.trim(),
        port: Number(form.port) || 0,
        key: form.key.trim(),
      });
      await mutateSource(next, { revalidate: false });
      setFields(null);
      // The viewer note on this page shows the address
      await mutate('/streaming/info');
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <section className="space-y-2">
      <h3 className="flex items-center gap-2 font-semibold">
        <Server className="size-4" />
        Stream server
      </h3>
      <div className="divide-y rounded-md border bg-card">
        <p className="px-4 py-3 text-muted-foreground">
          This instance is a viewer: it never asks the cameras to stream, and shows the stream server&apos;s video
          instead. Enter the stream server&apos;s address, and its stream password key if it has one turned on.
        </p>

        <form onSubmit={save} className="grid gap-4 px-4 py-4 sm:grid-cols-[1fr_7rem]">
          <div className="grid gap-2">
            <Label htmlFor="source-host">Host</Label>
            <Input
              id="source-host"
              value={form.host}
              onChange={(e) => set({ host: e.target.value })}
              placeholder="monitor.homelab.house"
              autoComplete="off"
              spellCheck={false}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="source-port">RTSP port</Label>
            <Input
              id="source-port"
              type="number"
              min={1}
              max={65535}
              value={form.port}
              onChange={(e) => set({ port: e.target.value })}
              placeholder="8554"
            />
          </div>
          <div className="grid gap-2 sm:col-span-2">
            <Label htmlFor="source-key">Key</Label>
            <div className="flex gap-2">
              <Input
                id="source-key"
                type={showKey ? 'text' : 'password'}
                value={form.key}
                onChange={(e) => set({ key: e.target.value })}
                placeholder="Leave empty if the server has no stream password"
                autoComplete="off"
                spellCheck={false}
                className={showKey ? 'font-mono' : undefined}
              />
              <Button
                type="button"
                size="icon"
                variant="outline"
                onClick={() => setShowKey(!showKey)}
                title={showKey ? 'Hide key' : 'Show key'}
                aria-label={showKey ? 'Hide key' : 'Show key'}
              >
                {showKey ? <EyeOff /> : <Eye />}
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              From the stream server&apos;s Settings → Streaming → Stream password.
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2 sm:col-span-2">
            <Button type="submit" size="sm" disabled={saving || !dirty}>
              {saving && <Loader2 className="animate-spin" />}
              Save and connect
            </Button>
            {dirty && (
              <Button type="button" size="sm" variant="ghost" onClick={() => setFields(null)} disabled={saving}>
                Discard changes
              </Button>
            )}
          </div>
        </form>

        {error && <NoteRow>Couldn&apos;t save: {error}</NoteRow>}

        {source.host ? (
          source.cameras.map((cam) => {
            const tried = cam.since > 0;
            return (
              <div key={cam.baby_uid} className="flex flex-wrap items-center justify-between gap-2 px-4 py-2">
                <span className="min-w-0">
                  <span className="font-medium">{displayName(cam)}</span>{' '}
                  <code className="font-mono text-xs text-muted-foreground">{cam.baby_uid}</code>
                  {cam.error && <span className="block text-sm text-muted-foreground">{cam.error}</span>}
                </span>
                <StatusLabel dot={cam.receiving ? DOT.good : tried ? DOT.bad : DOT.idle}>
                  {cam.receiving ? 'Receiving video' : tried ? 'Not receiving' : 'Connecting…'}
                </StatusLabel>
              </div>
            );
          })
        ) : (
          <NoteRow>No stream server set, so this instance has no video.</NoteRow>
        )}
      </div>
    </section>
  );
}
