import { api } from '@/lib/axios'
import type { AppPool, Site } from '@/types'

type PoolCommand = 'start' | 'stop' | 'restart' | 'recycle'
type SiteCommand = 'start' | 'stop' | 'restart'

interface ListParams {
  status?: 'Started' | 'Stopped'
  page?: number
  per_page?: number
}

export const appPoolService = {
  list: (projectKey: string, machineId: number, params?: ListParams) =>
    api.get<{ message: string; app_pools: AppPool[]; total: number }>(
      `/projects/${projectKey}/machines/${machineId}/app-pools`,
      { params },
    ),

  command: (projectKey: string, machineId: number, poolId: number, cmd: PoolCommand) =>
    api.post(`/projects/${projectKey}/machines/${machineId}/app-pools/${poolId}/${cmd}`),

  listSites: (projectKey: string, machineId: number, params?: ListParams) =>
    api.get<{ message: string; sites: Site[]; total: number }>(
      `/projects/${projectKey}/machines/${machineId}/sites`,
      { params },
    ),

  siteCommand: (projectKey: string, machineId: number, siteId: number, cmd: SiteCommand) =>
    api.post(`/projects/${projectKey}/machines/${machineId}/sites/${siteId}/${cmd}`),
}
