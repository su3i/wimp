import { api } from '@/lib/axios'
import type { Monitor } from '@/types'

interface CreateMonitorDTO {
  name: string
  url: string
  interval_seconds: number
}

export const monitorService = {
  list: (projectKey: string) =>
    api.get<{ monitors: Monitor[] }>(`/projects/${projectKey}/monitors`),

  create: (projectKey: string, dto: CreateMonitorDTO) =>
    api.post<{ monitor: Monitor }>(`/projects/${projectKey}/monitors`, dto),

  update: (projectKey: string, id: number, dto: CreateMonitorDTO) =>
    api.put<{ monitor: Monitor }>(`/projects/${projectKey}/monitors/${id}`, dto),

  delete: (projectKey: string, id: number) =>
    api.delete(`/projects/${projectKey}/monitors/${id}`),
}
