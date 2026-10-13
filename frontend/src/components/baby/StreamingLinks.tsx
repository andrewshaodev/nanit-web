import { useState } from 'react'
import { api } from '@/lib/api'
import { useStreamingInfo } from '@/hooks/useStreamingInfo'
import { copyToClipboard } from '@/lib/utils'
import { maskKey, useKeyRevealed } from '@/lib/keyReveal'
import type { Baby } from '@/types/api'
import { Cctv, Check, ChevronRight, Copy, Globe, Link, Radio, type LucideIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

interface StreamingLinksProps {
  baby: Baby
}

interface CopyButtonProps {
  text: string
  label: string
}

function CopyButton({ text, label }: CopyButtonProps) {
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    const success = await copyToClipboard(text)
    if (success) {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    }
  }

  return (
    <Button
      variant={copied ? 'success' : 'outline'}
      size="sm"
      onClick={handleCopy}
      className="w-full"
    >
      {copied ? <Check /> : <Copy />}
      {copied ? 'Copied!' : label}
    </Button>
  )
}

interface StreamCardProps {
  icon: LucideIcon
  title: string
  description: string
  url: string
  // What's shown, with the stream password's key masked; copying gives url
  displayUrl?: string
  copyLabel: string
  // Shown instead of the URL when this output is off
  disabled?: React.ReactNode
}

function StreamCard({ icon: Icon, title, description, url, displayUrl, copyLabel, disabled }: StreamCardProps) {
  return (
    // The URL and button sit at the bottom, so they line up across cards
    // whatever the length of each description
    <Card size="sm" className="h-full">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Icon className="size-4" />
          {title}
        </CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <CardContent className="mt-auto space-y-3">
        {disabled ? (
          <p className="rounded-md border bg-muted p-3 text-sm text-muted-foreground">{disabled}</p>
        ) : (
          <>
            <div className="bg-muted p-3 rounded-md border text-sm font-mono text-foreground overflow-x-auto whitespace-nowrap">
              {displayUrl ?? url}
            </div>
            <CopyButton text={url} label={copyLabel} />
          </>
        )}
      </CardContent>
    </Card>
  )
}

const envVar = (name: string) => <code className="font-mono text-foreground">{name}</code>

export default function StreamingLinks({ baby }: StreamingLinksProps) {
  const [isExpanded, setIsExpanded] = useState(false)

  // The RTSP and RTMP addresses come from the server (NANIT_RTMP_ADDR); they
  // are rarely the host and port the dashboard is served from
  const { streamingInfo } = useStreamingInfo(isExpanded)
  const rtspUrl = api.getRTSPUrl(baby.uid, streamingInfo)
  const rtmpUrl = api.getRTMPUrl(baby.uid, streamingInfo)
  const hlsUrl = api.getHLSUrl(baby.uid)
  const showKey = useKeyRevealed()
  const key = streamingInfo?.password?.enabled ? streamingInfo.password.key : undefined

  return (
    // A disclosure row, flush inside its CameraBox
    <div>
      <Button
        variant="ghost"
        onClick={() => setIsExpanded(!isExpanded)}
        aria-expanded={isExpanded}
        className="w-full h-auto px-4 py-2 rounded-none justify-between text-left font-normal hover:bg-accent aria-expanded:bg-transparent aria-expanded:hover:bg-accent"
      >
        <h3 className="font-medium flex items-center gap-2 text-muted-foreground">
          <Link />
          Streaming Links
        </h3>
        <ChevronRight className={`transition-transform duration-200 ${isExpanded ? 'rotate-90' : ''}`} />
      </Button>

      {isExpanded && (
        <div className="border-t p-4">
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            <StreamCard
              icon={Cctv}
              title="RTSP Stream"
              description="For Frigate, go2rtc, NVRs and Home Assistant"
              url={rtspUrl}
              displayUrl={maskKey(rtspUrl, key, showKey)}
              copyLabel="Copy RTSP URL"
              disabled={
                streamingInfo?.rtsp?.enabled === false && (
                  <>The RTSP output is disabled. Set {envVar('NANIT_RTSP_ENABLED=true')} to use it.</>
                )
              }
            />
            <StreamCard
              icon={Radio}
              title="RTMP Stream"
              description="For OBS, VLC and other RTMP players"
              url={rtmpUrl}
              displayUrl={maskKey(rtmpUrl, key, showKey)}
              copyLabel="Copy RTMP URL"
              disabled={
                streamingInfo?.rtmp?.enabled === false && (
                  <>
                    The built-in RTMP server is disabled. Set {envVar('NANIT_RTMP_ENABLED=true')} and{' '}
                    {envVar('NANIT_RTMP_ADDR')} to use it.
                  </>
                )
              }
            />
            <StreamCard
              icon={Globe}
              title="HLS Stream"
              description="For web browsers and modern apps"
              url={hlsUrl}
              copyLabel="Copy HLS URL"
            />
          </div>
        </div>
      )}
    </div>
  )
}
