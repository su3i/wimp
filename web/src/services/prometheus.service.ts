const BASE = import.meta.env.VITE_PROMETHEUS_URL as string | undefined

export interface PromRangeResult {
  metric: Record<string, string>
  values: [number, string][]
}

export interface PromInstantResult {
  metric: Record<string, string>
  value: [number, string]
}

export const prometheusService = {
  isConfigured: () => !!(BASE && BASE.trim()),

  range: async (query: string, start: number, end: number, step: number): Promise<PromRangeResult[]> => {
    const params = new URLSearchParams({ query, start: String(start), end: String(end), step: String(step) })
    const res = await fetch(`${BASE}/api/v1/query_range?${params}`)
    if (!res.ok) throw new Error(`Prometheus ${res.status}`)
    const json = await res.json() as { status: string; data: { result: PromRangeResult[] } }
    if (json.status !== 'success') throw new Error('Prometheus query failed')
    return json.data.result
  },

  instant: async (query: string): Promise<PromInstantResult[]> => {
    const params = new URLSearchParams({ query })
    const res = await fetch(`${BASE}/api/v1/query?${params}`)
    if (!res.ok) throw new Error(`Prometheus ${res.status}`)
    const json = await res.json() as { status: string; data: { result: PromInstantResult[] } }
    if (json.status !== 'success') throw new Error('Prometheus query failed')
    return json.data.result
  },
}
