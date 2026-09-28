import { useState } from 'react'
import { api } from '@/lib/api'
import { copyToClipboard } from '@/lib/utils'
import type { Baby } from '@/types/api'
import { Check, ChevronRight, Copy, Globe, Info, Link, Radio } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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

  const rtmpUrl = api.getRTMPUrl(baby.uid)
  const hlsUrl = api.getHLSUrl(baby.uid)

  return (
    <div className="border rounded-lg overflow-hidden">
      <Button
        variant="ghost"
        onClick={() => setIsExpanded(!isExpanded)}
        className="w-full h-auto px-4 py-3 rounded-none bg-muted justify-between text-left"
      >
        <h3 className="font-semibold flex items-center gap-2">
          <Link />
          Streaming Links
        </h3>
        <ChevronRight className={`transition-transform duration-200 ${isExpanded ? 'rotate-90' : ''}`} />
      </Button>

      {isExpanded && (
        <div className="p-4 space-y-4">
          <div className="grid md:grid-cols-2 gap-4">
            {/* RTMP Link */}
            <Card size="sm" className="border-l-4 border-l-ctp-red">
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
                <div className="bg-muted p-3 rounded-sm border text-sm font-mono text-foreground overflow-x-auto whitespace-nowrap">
                  {rtmpUrl}
                </div>
                <CopyButton text={rtmpUrl} label="Copy RTMP URL" />
              </CardContent>
            </Card>

            {/* HLS Link */}
            <Card size="sm" className="border-l-4 border-l-ctp-blue">
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

          <Alert variant="info">
            <Info />
            <AlertTitle>Usage Notes:</AlertTitle>
            <AlertDescription className="[&_p:not(:last-child)]:mb-1">
              <p>• RTMP streams work with most video software and Home Assistant</p>
              <p>• HLS streams work in web browsers and mobile apps</p>
              <p>• Start the video stream above before using these URLs</p>
            </AlertDescription>
          </Alert>
        </div>
      )}
    </div>
  )
}
