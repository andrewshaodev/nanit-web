import { useState } from 'react'
import { api } from '@/lib/api'
import { copyToClipboard } from '@/lib/utils'
import type { Baby } from '@/types/api'

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
    <button
      onClick={handleCopy}
      className={`btn text-sm w-full ${copied ? 'btn-success' : 'btn-primary'}`}
    >
      {copied ? '✅ Copied!' : `📋 ${label}`}
    </button>
  )
}

export default function StreamingLinks({ baby }: StreamingLinksProps) {
  const [isExpanded, setIsExpanded] = useState(false)

  const rtmpUrl = api.getRTMPUrl(baby.uid)
  const hlsUrl = api.getHLSUrl(baby.uid)

  return (
    <div className="border border-ctp-surface0 rounded-lg overflow-hidden">
      <button
        onClick={() => setIsExpanded(!isExpanded)}
        className="w-full px-4 py-3 bg-ctp-mantle hover:bg-ctp-surface0 transition-colors duration-200 flex items-center justify-between text-left"
      >
        <h3 className="font-semibold text-ctp-text flex items-center gap-2">
          🔗 Streaming Links
        </h3>
        <span className={`transform transition-transform duration-200 ${isExpanded ? 'rotate-90' : ''}`}>
          ▶
        </span>
      </button>
      
      {isExpanded && (
        <div className="p-4 space-y-4">
          <div className="grid md:grid-cols-2 gap-4">
            {/* RTMP Link */}
            <div className="bg-ctp-base border border-ctp-surface0 rounded-lg p-4 border-l-4 border-l-ctp-red">
              <h4 className="font-semibold text-ctp-text mb-2 flex items-center gap-2">
                📡 RTMP Stream
              </h4>
              <p className="text-sm text-ctp-subtext1 mb-3">
                For Home Assistant, OBS, VLC, etc.
              </p>
              <div className="bg-ctp-mantle p-3 rounded-sm border text-sm font-mono text-ctp-subtext1 mb-3 overflow-x-auto whitespace-nowrap">
                {rtmpUrl}
              </div>
              <CopyButton text={rtmpUrl} label="Copy RTMP URL" />
            </div>

            {/* HLS Link */}
            <div className="bg-ctp-base border border-ctp-surface0 rounded-lg p-4 border-l-4 border-l-ctp-blue">
              <h4 className="font-semibold text-ctp-text mb-2 flex items-center gap-2">
                🌐 HLS Stream
              </h4>
              <p className="text-sm text-ctp-subtext1 mb-3">
                For web browsers and modern apps
              </p>
              <div className="bg-ctp-mantle p-3 rounded-sm border text-sm font-mono text-ctp-subtext1 mb-3 overflow-x-auto whitespace-nowrap">
                {hlsUrl}
              </div>
              <CopyButton text={hlsUrl} label="Copy HLS URL" />
            </div>
          </div>

          <div className="bg-ctp-blue/10 border-l-4 border-ctp-blue p-3 rounded-sm">
            <div className="text-sm text-ctp-subtext1 space-y-1">
              <p><strong>Usage Notes:</strong></p>
              <p>• RTMP streams work with most video software and Home Assistant</p>
              <p>• HLS streams work in web browsers and mobile apps</p>
              <p>• Start the video stream above before using these URLs</p>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}