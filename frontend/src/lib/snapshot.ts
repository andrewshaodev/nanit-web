// Saves the frame a video is showing as a PNG download, and returns the
// file's name
export async function saveSnapshot(video: HTMLVideoElement): Promise<string> {
  if (video.readyState < 2) {
    throw new Error('Video not ready')
  }
  const canvas = document.createElement('canvas')
  canvas.width = video.videoWidth || video.clientWidth
  canvas.height = video.videoHeight || video.clientHeight
  const ctx = canvas.getContext('2d')
  if (!ctx) {
    throw new Error('Could not get canvas context')
  }
  ctx.drawImage(video, 0, 0, canvas.width, canvas.height)

  const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/png'))
  if (!blob) {
    throw new Error('Failed to create image')
  }
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `snapshot-${new Date().toISOString().slice(0, 19).replace(/:/g, '-')}.png`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
  return link.download
}
