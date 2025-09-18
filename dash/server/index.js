import express from 'express'
import pkg from 'pg'

const { Pool } = pkg

const app = express()
const port = process.env.API_PORT || 4000

// Basic CORS for local dev; Vite proxy will also be used.
// app.use((req, res, next) => {
//   res.setHeader('Access-Control-Allow-Origin', '*')
//   res.setHeader('Access-Control-Allow-Methods', 'GET,OPTIONS')
//   res.setHeader('Access-Control-Allow-Headers', 'Content-Type')
//   if (req.method === 'OPTIONS') return res.sendStatus(200)
//   next()
// })

// Prefer explicit env vars; otherwise use provided local credentials.
const pool = new Pool({
  host: process.env.PGHOST || 'localhost',
  port: +(process.env.PGPORT || 5432),
  database: process.env.PGDATABASE || 'gateway',
  user: process.env.PGUSER || 'gateway',
  password: process.env.PGPASSWORD || 'gateway_pass',
  ssl: process.env.PGSSL === 'true' ? { rejectUnauthorized: false } : false,
  // Add connection retry and pooling configuration
  connectionTimeoutMillis: 5000,
  max: 20,
  idleTimeoutMillis: 30000,
  statement_timeout: 30000,
  query_timeout: 30000,
})

app.get('/api/health', async (_req, res) => {
  try {
    // Test database connection
    await pool.query('SELECT 1')
    res.json({
      ok: true,
      database: 'connected',
      timestamp: new Date().toISOString()
    })
  } catch (err) {
    console.error('[server] Health check failed:', err.message)
    res.status(503).json({
      ok: false,
      database: 'disconnected',
      error: err.message,
      timestamp: new Date().toISOString()
    })
  }
})

app.get('/api/request-logs', async (req, res) => {
  const limit = Math.min(parseInt(req.query.limit) || 25, 200)
  const page = Math.max(parseInt(req.query.page) || 1, 1)
  const offset = (page - 1) * limit

  try {
    const client = await pool.connect()
    try {
      const countResult = await client.query('SELECT COUNT(*)::int AS count FROM request_logs')
      const total = countResult.rows[0]?.count ?? 0

      const rowsResult = await client.query(
        `SELECT id, timestamp, request_id, endpoint, method, status_code, latency_ms, provider
         FROM request_logs
         ORDER BY timestamp DESC
         LIMIT $1 OFFSET $2`,
        [limit, offset]
      )

      res.json({
        page,
        limit,
        total,
        rows: rowsResult.rows,
      })
    } finally {
      client.release()
    }
  } catch (err) {
    console.error('[server] /api/request-logs error', err)
    res.status(500).json({ error: 'Failed to fetch logs' })
  }
})

app.get('/api/guardrail-metrics', async (req, res) => {
  const limit = Math.min(parseInt(req.query.limit) || 25, 200)
  const page = Math.max(parseInt(req.query.page) || 1, 1)
  const offset = (page - 1) * limit

  try {
    const client = await pool.connect()
    try {
      const countResult = await client.query(
        'SELECT COUNT(*)::int AS count FROM guardrail_metrics'
      )
      const total = countResult.rows[0]?.count ?? 0

      const rowsResult = await client.query(
        `SELECT id, request_id, guardrail_name, layer, priority, start_time, end_time, duration_ms, passed, score, error, metadata, original_response, override_response, response_overridden, created_at
         FROM guardrail_metrics
         ORDER BY start_time DESC
         LIMIT $1 OFFSET $2`,
        [limit, offset]
      )

      res.json({ page, limit, total, rows: rowsResult.rows })
    } finally {
      client.release()
    }
  } catch (err) {
    console.error('[server] /api/guardrail-metrics error', err)
    res.status(500).json({ error: 'Failed to fetch guardrail metrics' })
  }
})

// Fetch a single request log by id with all columns
app.get('/api/request-logs/:id', async (req, res) => {
  const { id } = req.params
  try {
    const client = await pool.connect()
    try {
      // Support lookup by either primary key id or external request_id
      const result = await client.query(
        'SELECT * FROM request_logs WHERE request_id::text = $1 LIMIT 1',
        [id]
      )
      if (result.rows.length === 0) return res.status(404).json({ error: 'Not found' })
      res.json(result.rows[0])
    } finally {
      client.release()
    }
  } catch (err) {
    console.error('[server] /api/request-logs/:id error', err)
    res.status(500).json({ error: 'Failed to fetch log' })
  }
})

// Add graceful shutdown handling
process.on('SIGTERM', async () => {
  console.log('SIGTERM received, shutting down gracefully...')
  await pool.end()
  process.exit(0)
})

process.on('SIGINT', async () => {
  console.log('SIGINT received, shutting down gracefully...')
  await pool.end()
  process.exit(0)
})

// Add error handling for uncaught exceptions
process.on('uncaughtException', (err) => {
  console.error('Uncaught Exception:', err)
  process.exit(1)
})

process.on('unhandledRejection', (reason, promise) => {
  console.error('Unhandled Rejection at:', promise, 'reason:', reason)
  process.exit(1)
})

app.listen(port, '0.0.0.0', () => {
  console.log(`🚀 [server] Dashboard API listening on http://0.0.0.0:${port}`)
  console.log(`📊 Health check available at http://0.0.0.0:${port}/api/health`)
})
