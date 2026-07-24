const streamUrl = '/api/v1/jobs/stream'
let lastEventId = sessionStorage.getItem('job-stream:last-event-id')

async function connectJobsStream(signal: AbortSignal): Promise<void> {
  const headers: Record<string, string> = { Accept: 'text/event-stream' }
  if (lastEventId) headers['Last-Event-ID'] = lastEventId

  const response = await fetch(streamUrl, { headers, cache: 'no-store', signal })
  if (!response.ok || !response.body) throw new Error(`SSE HTTP ${response.status}`)

  const reader = response.body.pipeThrough(new TextDecoderStream()).getReader()
  let buffer = ''
  for (;;) {
    const { value, done } = await reader.read()
    if (done) break
    buffer += value
    for (;;) {
      const boundary = buffer.indexOf('\n\n')
      if (boundary < 0) break
      const raw = buffer.slice(0, boundary)
      buffer = buffer.slice(boundary + 2)
      handleFrame(raw)
    }
  }
}

function handleFrame(raw: string): void {
  const frame: { id?: string; event?: string; data?: string } = {}
  for (const line of raw.split('\n')) {
    if (line.startsWith('id:')) frame.id = line.slice(3).trim()
    if (line.startsWith('event:')) frame.event = line.slice(6).trim()
    if (line.startsWith('data:')) frame.data = line.slice(5).trim()
  }
  if (frame.event === 'heartbeat') return
  if (frame.event === 'stream.resync_required') {
    const data = JSON.parse(frame.data ?? '{}') as {
      high_watermark_event_id: number
      active_jobs_url: '/api/v1/jobs?active=true'
    }
    lastEventId = String(data.high_watermark_event_id)
    sessionStorage.setItem('job-stream:last-event-id', lastEventId)
    void fetch(data.active_jobs_url, { cache: 'no-store' })
    return
  }
  if (frame.id) {
    lastEventId = frame.id
    sessionStorage.setItem('job-stream:last-event-id', frame.id)
  }
  // Parse frame.data as JobStreamData and merge it into the frontend cache.
}
