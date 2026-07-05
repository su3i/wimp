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

  // Stages the zipped logs on the control plane; server-side zip+transfer from the
  // agent can take a while, so this gets a generous timeout independent of the
  // client's default.
  stageDownload: (projectKey: string, machineId: number, logPath: string) =>
    api.get<{ message: string; token: string; file_name: string; file_size: number }>(
      `/projects/${projectKey}/machines/${machineId}/logs/download`,
      { params: { path: logPath }, timeout: 120_000 },
    ),

  fetchDownload: (token: string) =>
    api.get(`/downloads/${token}`, { responseType: 'blob' }),
}
