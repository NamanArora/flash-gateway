import { useEffect, useMemo, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'


type Role = 'user' | 'assistant'

type ChatMessage = {
  id: string
  role: Role
  content: string
}

export default function Playground() {
  const [proxyUrl, setProxyUrl] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [endpointMode, setEndpointMode] = useState<'responses' | 'chat'>('responses')
  const defaultSystemPrompt = `You are a helpful assistant.`
  const [systemPrompt, setSystemPrompt] = useState<string>('')
  const [connected, setConnected] = useState(false)
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [input, setInput] = useState('')
  const [sending, setSending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const messagesEndRef = useRef<HTMLDivElement | null>(null)

  // Load persisted endpoint/apiKey and chat
  useEffect(() => {
    const savedProxy =
      localStorage.getItem('playground:proxyUrl') ||
      // Back-compat: migrate old stored endpoint to proxyUrl if present.
      localStorage.getItem('playground:endpoint') ||
      ''
    const savedKey = localStorage.getItem('playground:apiKey') || ''
    const savedMode = (localStorage.getItem('playground:endpointMode') as 'responses' | 'chat') || 'responses'
    const savedMsgs = localStorage.getItem('playground:messages')
    const savedSystem = localStorage.getItem('playground:systemPrompt') || ''
    setProxyUrl(savedProxy)
    setApiKey(savedKey)
    setEndpointMode(savedMode)
    setSystemPrompt(savedSystem || defaultSystemPrompt)
    if (savedMsgs) {
      try {
        const parsed = JSON.parse(savedMsgs) as ChatMessage[]
        setMessages(parsed)
      } catch (err) {
        console.error('Failed to parse saved messages', err)
      }
    }
    if (savedProxy && savedKey) setConnected(true)
  }, [])

  useEffect(() => {
    localStorage.setItem('playground:messages', JSON.stringify(messages))
  }, [messages])

  // Auto-scroll to bottom when messages change
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, sending])

  const canSend = useMemo(() => connected && !sending && input.trim().length > 0, [connected, sending, input])

  function handleConnect() {
    setError(null)
    if (!proxyUrl.trim() || !apiKey.trim()) {
      setError('Please provide both endpoint and API key')
      return
    }
    localStorage.setItem('playground:proxyUrl', proxyUrl.trim())
    localStorage.setItem('playground:apiKey', apiKey.trim())
    localStorage.setItem('playground:endpointMode', endpointMode)
    localStorage.setItem('playground:systemPrompt', systemPrompt)
    setConnected(true)
  }

  function handleDisconnect() {
    setConnected(false)
  }

  function resetChat() {
    setMessages([])
    setError(null)
  }

  async function sendMessage() {
    if (!canSend) return
    const content = input.trim()
    setInput('')
    const userMsg: ChatMessage = { id: crypto.randomUUID(), role: 'user', content }
    setMessages((prev) => [...prev, userMsg])
    setSending(true)
    setError(null)

    try {
      // Build endpoint and payload depending on selected mode
      const base = proxyUrl.replace(/\/?$/, '')
      const url = endpointMode === 'responses' ? `${base}/v1/responses` : `${base}/v1/chat/completions`

      let body: Record<string, unknown>
      if (endpointMode === 'responses') {
        const inputText = [
          ...messages.map((m) => `${m.role}: ${m.content}`),
          `user: ${content}`,
        ].join('\n')
        body = {
          model: 'gpt-4o-mini',
          input: inputText,
          instructions: systemPrompt,
          stream: false,
        }
      } else {
        // chat completions
        const chatMessages = [
          ...(systemPrompt ? [{ role: 'system', content: systemPrompt }] : []),
          ...messages.map((m) => ({ role: m.role, content: m.content })),
          { role: 'user', content },
        ]
        body = {
          model: 'gpt-4o-mini',
          messages: chatMessages,
          stream: false,
        }
      }

      // Log prettified request
      try {
        const redactedKey = apiKey ? `${apiKey.slice(0, 6)}…${apiKey.slice(-4)}` : ''
        console.groupCollapsed('[Playground] Request')
        console.log('URL:', url)
        console.log('Headers:', { 'Content-Type': 'application/json', Authorization: `Bearer ${redactedKey}` })
        console.log('Body:', JSON.stringify(body, null, 2))
        console.groupEnd()
      } catch (logErr) {
        console.warn('Failed to log request', logErr)
      }

      const res = await fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${apiKey}`,
        },
        body: JSON.stringify(body),
      })

      if (!res.ok) {
        const text = await res.text().catch(() => '')
        try {
          console.groupCollapsed('[Playground] Response (error)')
          console.log('Status:', res.status, res.statusText)
          console.log('Body:', text)
          console.groupEnd()
        } catch {}
        throw new Error(`Request failed: ${res.status} ${res.statusText}${text ? ` - ${text}` : ''}`)
      }

      const data = await res.json()
      try {
        console.groupCollapsed('[Playground] Response')
        console.log('Status:', res.status)
        console.log('Body:', JSON.stringify(data, null, 2))
        console.groupEnd()
      } catch {}
      // Parse response flexibly
      let assistantText = ''
      if (typeof data?.output_text === 'string' && data.output_text) {
        assistantText = data.output_text
      } else if (Array.isArray(data?.output) && data.output.length > 0) {
        // Concatenate any text parts from output
        const parts: string[] = []
        for (const item of data.output) {
          const content = item?.content
          if (Array.isArray(content)) {
            for (const c of content) {
              if (typeof c?.text === 'string') parts.push(c.text)
            }
          }
        }
        assistantText = parts.join('')
      } else if (data?.choices?.[0]?.message?.content) {
        assistantText = String(data.choices[0].message.content)
      } else if (typeof data?.content === 'string') {
        // Fallback to simple { content } shape
        assistantText = data.content
      } else if (typeof data?.text === 'string') {
        assistantText = data.text
      } else {
        assistantText = JSON.stringify(data)
      }

      const assistantMsg: ChatMessage = { id: crypto.randomUUID(), role: 'assistant', content: assistantText }
      setMessages((prev) => [...prev, assistantMsg])
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'Failed to get response'
      setError(msg)
    } finally {
      setSending(false)
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-xl font-semibold">Playground</h2>
        {connected ? (
          <div className="flex items-center gap-2">
            <Button variant="outline" onClick={resetChat} disabled={sending}>Reset</Button>
            <Button variant="outline" onClick={handleDisconnect} disabled={sending}>Disconnect</Button>
          </div>
        ) : null}
      </div>

      {!connected && (
        <div className="rounded-lg border p-4 space-y-3">
          <div className="space-y-1">
            <label className="text-sm font-medium">Proxy URL</label>
            <Input
              placeholder="https://your-proxy.example.com"
              value={proxyUrl}
              onChange={(e) => setProxyUrl(e.target.value)}
            />
          </div>
          <div className="space-y-1">
            <label className="text-sm font-medium">Endpoint</label>
            <select
              className="h-8 w-full rounded-md border bg-background px-2 text-sm"
              value={endpointMode}
              onChange={(e) => setEndpointMode(e.target.value as 'responses' | 'chat')}
            >
              <option value="responses">Responses API (/v1/responses)</option>
              <option value="chat">Chat Completions (/v1/chat/completions)</option>
            </select>
          </div>
          <div className="space-y-1">
            <label className="text-sm font-medium">System Prompt</label>
            <textarea
              className="w-full min-h-28 rounded-md border bg-background p-2 text-sm"
              value={systemPrompt}
              onChange={(e) => setSystemPrompt(e.target.value)}
              placeholder="Enter a system prompt for the assistant"
            />
            <p className="text-xs text-muted-foreground">Default provided. You can override before starting a new chat.</p>
          </div>
          <div className="space-y-1">
            <label className="text-sm font-medium">API Key</label>
            <Input
              type="password"
              placeholder="sk-..."
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
            />
          </div>
          {error && <div className="text-sm text-red-600 dark:text-red-400">{error}</div>}
          <div className="flex gap-2">
            <Button onClick={handleConnect}>Save & Start</Button>
          </div>
          <p className="text-xs text-muted-foreground">
            Uses OpenAI endpoints via your proxy. Requests go to
            <code className="font-mono"> {proxyUrl.replace(/\/?$/, '')}{endpointMode === 'responses' ? '/v1/responses' : '/v1/chat/completions'}</code>.
          </p>
        </div>
      )}

      {connected && (
        <div className="rounded-lg border flex flex-col h-[70vh]">
          <div className="p-3">
            <div className="text-sm text-muted-foreground">
              Connected to
              <span className="font-medium"> {proxyUrl.replace(/\/?$/, '')}{endpointMode === 'responses' ? '/v1/responses' : '/v1/chat/completions'}</span>
            </div>
          </div>
          <Separator />
          <div className="flex-1 overflow-auto p-4 space-y-3">
            {messages.length === 0 && (
              <div className="text-sm text-muted-foreground">Say hello to start the conversation.</div>
            )}
            {messages.map((m) => (
              <div
                key={m.id}
                className={
                  m.role === 'user'
                    ? 'flex justify-end'
                    : 'flex justify-start'
                }
              >
                <div
                  className={
                    'max-w-[85%] rounded-lg px-3 py-2 text-sm ' +
                    (m.role === 'user'
                      ? 'bg-primary text-primary-foreground'
                      : 'bg-muted text-foreground')
                  }
                >
                  {m.content}
                </div>
              </div>
            ))}
            {sending && (
              <div className="text-xs text-muted-foreground">Thinking…</div>
            )}
            <div ref={messagesEndRef} />
          </div>
          <Separator />
          <div className="p-3 border-t flex gap-2">
            <Input
              placeholder="Type your message"
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault()
                  sendMessage()
                }
              }}
              disabled={!connected || sending}
            />
            <Button onClick={sendMessage} disabled={!canSend}>Send</Button>
          </div>
          {error && connected && (
            <div className="p-3 text-xs text-red-600 dark:text-red-400 border-t">{error}</div>
          )}
        </div>
      )}
    </div>
  )
}
