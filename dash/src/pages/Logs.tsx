import { useEffect, useMemo, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { fetchRequestLogs, fetchRequestLog, type RequestLog, type RequestLogDetails } from '@/lib/api'
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'

export default function Logs() {
  const [logs, setLogs] = useState<RequestLog[]>([])
  const [page, setPage] = useState(1)
  const [limit] = useState(25)
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [details, setDetails] = useState<RequestLogDetails | null>(null)
  const [detailsLoading, setDetailsLoading] = useState(false)
  const [detailsError, setDetailsError] = useState<string | null>(null)

  const totalPages = useMemo(() => Math.max(1, Math.ceil(total / limit)), [total, limit])

  const load = async (p = page) => {
    setLoading(true)
    setError(null)
    try {
      const data = await fetchRequestLogs({ page: p, limit })
      setLogs(data.rows)
      setTotal(data.total)
    } catch (e: any) {
      setError(e?.message || 'Failed to load logs')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load(1)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (page !== 1) load(page)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page])

  useEffect(() => {
    async function loadDetails(id: string) {
      setDetails(null)
      setDetailsError(null)
      setDetailsLoading(true)
      try {
        const data = await fetchRequestLog(id)
        setDetails(data)
      } catch (e: unknown) {
        const msg = e instanceof Error ? e.message : 'Failed to load details'
        setDetailsError(msg)
      } finally {
        setDetailsLoading(false)
      }
    }
    if (selectedId) loadDetails(selectedId)
  }, [selectedId])

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-xl font-semibold">Request Logs</h2>
        <div className="flex items-center gap-2">
          <Button variant="outline" onClick={() => load(page)}>Refresh</Button>
        </div>
      </div>

      {error && (
        <div className="text-sm text-red-600 dark:text-red-400">{error}</div>
      )}

      <div className="rounded-md border">
        <Table className="table-auto">
          <TableHeader>
            <TableRow>
              <TableHead className="w-[180px]">Timestamp</TableHead>
              <TableHead className="w-[80px]">Method</TableHead>
              <TableHead className="w-[45%]">Endpoint</TableHead>
              <TableHead className="w-[80px]">Status</TableHead>
              <TableHead className="w-[110px]">Latency (ms)</TableHead>
              <TableHead className="w-[120px]">Provider</TableHead>
              <TableHead className="w-[280px]">Request ID</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading
              ? Array.from({ length: 8 }).map((_, i) => (
                  <TableRow key={i}>
                    <TableCell><Skeleton className="h-4 w-40" /></TableCell>
                    <TableCell><Skeleton className="h-4 w-12" /></TableCell>
                    <TableCell><Skeleton className="h-4 w-80" /></TableCell>
                    <TableCell><Skeleton className="h-4 w-10" /></TableCell>
                    <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                    <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                    <TableCell><Skeleton className="h-4 w-56" /></TableCell>
                  </TableRow>
                ))
              : logs.map((row) => (
                  <TableRow key={row.id} className="cursor-pointer" onClick={() => setSelectedId(row.request_id)}>
                    <TableCell className="whitespace-nowrap">{new Date(row.timestamp).toLocaleString()}</TableCell>
                    <TableCell className="font-medium whitespace-nowrap">{row.method}</TableCell>
                    <TableCell className="truncate" title={row.endpoint}>{row.endpoint}</TableCell>
                    <TableCell className="whitespace-nowrap">{row.status_code ?? '-'}</TableCell>
                    <TableCell className="whitespace-nowrap">{row.latency_ms ?? '-'}</TableCell>
                    <TableCell className="whitespace-nowrap">{row.provider ?? '-'}</TableCell>
                    <TableCell className="font-mono text-xs truncate" title={row.request_id}>{row.request_id}</TableCell>
                  </TableRow>
                ))}
          </TableBody>
          <TableCaption>
            Showing page {page} of {totalPages} • {total} total
          </TableCaption>
        </Table>
      </div>

      <div className="flex items-center justify-between">
        <div className="text-xs text-muted-foreground">Limit: {limit}</div>
        <div className="flex gap-2">
          <Button variant="outline" disabled={page <= 1 || loading} onClick={() => setPage((p) => Math.max(1, p - 1))}>
            Previous
          </Button>
          <Button variant="outline" disabled={page >= totalPages || loading} onClick={() => setPage((p) => Math.min(totalPages, p + 1))}>
            Next
          </Button>
        </div>
      </div>
      <Sheet open={!!selectedId} onOpenChange={(open) => !open && setSelectedId(null)}>
        <SheetContent side="right" className="sm:max-w-xl">
          <SheetHeader>
            <SheetTitle>Request Details</SheetTitle>
          </SheetHeader>
          <div className="p-4 space-y-3 overflow-auto">
            {detailsLoading && <div className="text-sm text-muted-foreground">Loading…</div>}
            {detailsError && <div className="text-sm text-red-600 dark:text-red-400">{detailsError}</div>}
            {details && (
              <div className="space-y-2">
                {Object.entries(details).map(([key, value]) => (
                  <div key={key} className="text-sm">
                    <span className="text-muted-foreground mr-2">{key}:</span>
                    <span className="font-mono break-all">
                      {typeof value === 'object' ? JSON.stringify(value) : String(value)}
                    </span>
                  </div>
                ))}
                <div className="pt-2">
                  <div className="text-muted-foreground text-xs mb-1">Raw</div>
                  <pre className="bg-muted rounded-md p-3 text-xs overflow-auto">
{JSON.stringify(details, null, 2)}
                  </pre>
                </div>
              </div>
            )}
          </div>
        </SheetContent>
      </Sheet>
    </div>
  )
}
