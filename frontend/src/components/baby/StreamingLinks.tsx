import { useState } from 'react'
import { api } from '@/lib/api'
import { useStreamingInfo } from '@/hooks/useStreamingInfo'
import { copyToClipboard } from '@/lib/utils'
import type { Baby } from '@/types/api'
import { Check, ChevronRight, Copy, Globe, Link, Radio } from 'lucide-react'
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

export default function StreamingLinks({ baby }: StreamingLinksProps) {
  const [isExpanded, setIsExpanded] = useState(false)

  // The RTMP address comes from the server (NANIT_RTMP_ADDR); it's rarely the
  // host and port the dashboard is served from
  const { streamingInfo } = useStreamingInfo(isExpanded)
  const rtmpUrl = api.getRTMPUrl(baby.uid, streamingInfo)
  const rtmpDisabled = streamingInfo?.rtmp?.enabled === false
  const hlsUrl = api.getHLSUrl(baby.uid)

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
        <div className="border-t p-4 space-y-4">
          <div className="grid md:grid-cols-2 gap-4">
            {/* RTMP Link */}
            <Card size="sm">
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Radio className="size-4" />
                  RTMP Stream
                </CardTitle>
                <CardDescription>
                  For Home Assistant, OBS, VLC, etc.
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                {rtmpDisabled ? (
                  <p className="rounded-md border bg-muted p-3 text-sm text-muted-foreground">
                    The built-in RTMP server is disabled. Set{' '}
                    <code className="font-mono text-foreground">NANIT_RTMP_ENABLED=true</code> and{' '}
                    <code className="font-mono text-foreground">NANIT_RTMP_ADDR</code> to use it.
                  </p>
                ) : (
                  <>
                    <div className="bg-muted p-3 rounded-md border text-sm font-mono text-foreground overflow-x-auto whitespace-nowrap">
                      {rtmpUrl}
                    </div>
                    <CopyButton text={rtmpUrl} label="Copy RTMP URL" />
                  </>
                )}
              </CardContent>
            </Card>

            {/* HLS Link */}
            <Card size="sm">
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Globe className="size-4" />
                  HLS Stream
                </CardTitle>
                <CardDescription>
                  For web browsers and modern apps
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="bg-muted p-3 rounded-sm border text-sm font-mono text-foreground overflow-x-auto whitespace-nowrap">
                  {hlsUrl}
                </div>
                <CopyButton text={hlsUrl} label="Copy HLS URL" />
              </CardContent>
            </Card>
          </div>

        </div>
      )}
    </div>
  )
}
