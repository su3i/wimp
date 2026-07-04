import { api } from '@/lib/axios'
import type { LokiResponse } from '@/types'

export interface LogQueryParams {
  start?: string
  end?: string
  limit?: number
  direction?: string
  machine_id?: number
  pool_id?: number
  filename?: string
}

export const logsService = {
  query: (projectKey: string, appId: number, params: LogQueryParams) =>
    api.get<LokiResponse>(`/projects/${projectKey}/applications/${appId}/logs`, { params }),

  listFiles: (projectKey: string, appId: number) =>
    api.get<{ message: string; files: string[] }>(`/projects/${projectKey}/applications/${appId}/files`),

  downloadLogs: (projectKey: string, machineId: number, logPath: string) =>
    api.get(`/projects/${projectKey}/machines/${machineId}/logs/download`, {
      params: { path: logPath },
      responseType: 'blob',
    }),
}
