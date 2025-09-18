export type RequestLog = {
  id: string
  timestamp: string
  request_id: string
  endpoint: string
  method: string
  status_code: number | null
  latency_ms: number | null
  provider: string | null
}

export type RequestLogsResponse = {
  page: number
  limit: number
  total: number
  rows: RequestLog[]
}

export async function fetchRequestLogs(params?: { page?: number; limit?: number }) {
  const page = params?.page ?? 1
  const limit = params?.limit ?? 25
  const res = await fetch(`/api/request-logs?page=${page}&limit=${limit}`)
  if (!res.ok) throw new Error(`Failed to fetch logs: ${res.status}`)
  return (await res.json()) as RequestLogsResponse
}

export type RequestLogDetails = Record<string, unknown>

export async function fetchRequestLog(id: string) {
  const res = await fetch(`/api/request-logs/${encodeURIComponent(id)}`)
  if (!res.ok) throw new Error(`Failed to fetch log ${id}: ${res.status}`)
  return (await res.json()) as RequestLogDetails
}

export type GuardrailMetric = {
  id: string
  request_id: string
  guardrail_name: string
  layer: 'input' | 'output'
  priority: number
  start_time: string
  end_time: string
  duration_ms: number
  passed: boolean
  score: number | null
  error: string | null
  metadata: unknown
  original_response: string | null
  override_response: string | null
  response_overridden: boolean
  created_at: string
}

export type GuardrailMetricsResponse = {
  page: number
  limit: number
  total: number
  rows: GuardrailMetric[]
}

export async function fetchGuardrailMetrics(params?: { page?: number; limit?: number }) {
  const page = params?.page ?? 1
  const limit = params?.limit ?? 25
  const res = await fetch(`/api/guardrail-metrics?page=${page}&limit=${limit}`)
  if (!res.ok) throw new Error(`Failed to fetch guardrail metrics: ${res.status}`)
  return (await res.json()) as GuardrailMetricsResponse
}
