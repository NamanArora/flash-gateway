import { useEffect, useMemo, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { fetchGuardrailMetrics, type GuardrailMetric } from '@/lib/api'
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'

export default function Guardrail() {
  const [rows, setRows] = useState<GuardrailMetric[]>([])
  const [page, setPage] = useState(1)
  const [limit] = useState(25)
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selected, setSelected] = useState<GuardrailMetric | null>(null)

  const totalPages = useMemo(() => Math.max(1, Math.ceil(total / limit)), [total, limit])

  const load = async (p = page) => {
    setLoading(true)
    setError(null)
    try {
      const data = await fetchGuardrailMetrics({ page: p, limit })
      setRows(data.rows)
      setTotal(data.total)
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'Failed to load metrics'
      setError(msg)
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

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-xl font-semibold">Guardrail Metrics</h2>
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
              <TableHead className="w-[180px]">Start Time</TableHead>
              <TableHead className="w-[160px]">Guardrail</TableHead>
              <TableHead className="w-[100px]">Layer</TableHead>
              <TableHead className="w-[90px]">Priority</TableHead>
              <TableHead className="w-[90px]">Passed</TableHead>
              <TableHead className="w-[100px]">Score</TableHead>
              <TableHead className="w-[120px]">Duration (ms)</TableHead>
              <TableHead className="w-[120px]">Response Override</TableHead>
              <TableHead className="w-[280px]">Request ID</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading
              ? Array.from({ length: 8 }).map((_, i) => (
                  <TableRow key={i}>
                    <TableCell><Skeleton className="h-4 w-40" /></TableCell>
                    <TableCell><Skeleton className="h-4 w-32" /></TableCell>
                    <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                    <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                    <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                    <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                    <TableCell><Skeleton className="h-4 w-20" /></TableCell>
                    <TableCell><Skeleton className="h-4 w-20" /></TableCell>
                    <TableCell><Skeleton className="h-4 w-56" /></TableCell>
                  </TableRow>
                ))
              : rows.map((r) => (
                  <TableRow 
                    key={r.id} 
                    className={`cursor-pointer ${!r.passed ? 'bg-red-50 dark:bg-red-950/20 hover:bg-red-100 dark:hover:bg-red-950/30' : 'hover:bg-muted/50'}`} 
                    onClick={() => setSelected(r)}
                  >
                    <TableCell className="whitespace-nowrap">{new Date(r.start_time).toLocaleString()}</TableCell>
                    <TableCell className="font-medium">{r.guardrail_name}</TableCell>
                    <TableCell className="whitespace-nowrap capitalize">{r.layer}</TableCell>
                    <TableCell className="whitespace-nowrap">{r.priority}</TableCell>
                    <TableCell className="whitespace-nowrap">
                      <span className={r.passed ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}>
                        {r.passed ? '✓ Yes' : '✗ No'}
                      </span>
                    </TableCell>
                    <TableCell className="whitespace-nowrap">{r.score ?? '-'}</TableCell>
                    <TableCell className="whitespace-nowrap">{r.duration_ms}</TableCell>
                    <TableCell className="whitespace-nowrap">
                      {r.response_overridden ? (
                        <span className="text-amber-600 dark:text-amber-400 bg-amber-100 dark:bg-amber-900/30 px-2 py-1 rounded-sm text-xs font-medium">
                          Override
                        </span>
                      ) : (
                        <span className="text-gray-400">-</span>
                      )}
                    </TableCell>
                    <TableCell className="font-mono text-xs truncate" title={r.request_id}>{r.request_id}</TableCell>
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

      <Sheet open={!!selected} onOpenChange={(open) => !open && setSelected(null)}>
        <SheetContent side="right" className="sm:max-w-xl">
          <SheetHeader>
            <SheetTitle>Guardrail Details</SheetTitle>
          </SheetHeader>
          <div className="p-4 space-y-2 overflow-auto">
            {selected && (
              <div className="space-y-2 text-sm">
                <KV label="Guardrail" value={selected.guardrail_name} />
                <KV label="Layer" value={selected.layer} />
                <KV label="Priority" value={String(selected.priority)} />
                <KV label="Passed" value={selected.passed ? 'Yes' : 'No'} />
                <KV label="Score" value={selected.score != null ? String(selected.score) : '-'} />
                <KV label="Duration (ms)" value={String(selected.duration_ms)} />
                <KV label="Response Overridden" value={selected.response_overridden ? 'Yes' : 'No'} />
                <KV label="Start" value={new Date(selected.start_time).toLocaleString()} />
                <KV label="End" value={new Date(selected.end_time).toLocaleString()} />
                <KV label="Request ID" mono value={selected.request_id} />
                
                {selected.error && (
                  <div>
                    <div className="text-muted-foreground text-xs mb-1">Error</div>
                    <pre className="bg-muted rounded-md p-3 text-xs whitespace-pre-wrap break-words">{selected.error}</pre>
                  </div>
                )}
                
                {selected.original_response && (
                  <div>
                    <div className="text-muted-foreground text-xs mb-1">Original Response (Blocked)</div>
                    <pre className="bg-red-50 dark:bg-red-950/30 border border-red-200 dark:border-red-800 rounded-md p-3 text-xs whitespace-pre-wrap break-words max-h-48 overflow-auto">{selected.original_response}</pre>
                  </div>
                )}
                
                {selected.override_response && (
                  <div>
                    <div className="text-muted-foreground text-xs mb-1">Override Response (Sent to User)</div>
                    <pre className="bg-green-50 dark:bg-green-950/30 border border-green-200 dark:border-green-800 rounded-md p-3 text-xs whitespace-pre-wrap break-words max-h-48 overflow-auto">{selected.override_response}</pre>
                  </div>
                )}
                
                {selected.metadata != null && (
                  <div>
                    <div className="text-muted-foreground text-xs mb-1">Metadata</div>
                    <pre className="bg-muted rounded-md p-3 text-xs overflow-auto">{JSON.stringify(selected.metadata, null, 2)}</pre>
                  </div>
                )}
              </div>
            )}
          </div>
        </SheetContent>
      </Sheet>
    </div>
  )
}

function KV({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <span className="text-muted-foreground mr-2">{label}:</span>
      <span className={mono ? 'font-mono break-all' : ''}>{value}</span>
    </div>
  )
}
