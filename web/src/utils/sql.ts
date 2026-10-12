import type { SqlInstance } from '@/types'

export function instanceLabel(inst: Pick<SqlInstance, 'Name'>) {
  return inst.Name === 'MSSQLSERVER' ? 'MSSQLSERVER (default)' : inst.Name
}

export function apiError(err: unknown, fallback: string) {
  return (err as { response?: { data?: { error?: string } } })?.response?.data?.error ?? fallback
}
