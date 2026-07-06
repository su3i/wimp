import { api } from '@/lib/axios'
import type { Application, ApplicationDetail } from '@/types'

type PoolCommand = 'start' | 'stop' | 'restart' | 'recycle'

export const applicationService = {
  create: (projectKey: string, name: string, healthCheckURL?: string | null, healthCheckIntervalSeconds?: number) =>
    api.post<{ message: string; application: Application }>(
      `/projects/${projectKey}/applications`,
      { name, health_check_url: healthCheckURL ?? null, health_check_interval_seconds: healthCheckIntervalSeconds ?? 60 },
    ),

  update: (projectKey: string, appId: number, data: { name: string; health_check_url?: string | null; health_check_interval_seconds?: number }) =>
    api.put<{ message: string; application: Application }>(
      `/projects/${projectKey}/applications/${appId}`,
      data,
    ),

  list: (projectKey: string, params?: { page?: number; per_page?: number }) =>
    api.get<{ message: string; applications: Application[]; total: number; page: number; per_page: number }>(
      `/projects/${projectKey}/applications`,
      { params },
    ),

  get: (projectKey: string, appId: number) =>
    api.get<{ message: string; application: ApplicationDetail }>(
      `/projects/${projectKey}/applications/${appId}`,
    ),

  poolCommand: (projectKey: string, machineId: number, poolId: number, cmd: PoolCommand) =>
    api.post(`/projects/${projectKey}/machines/${machineId}/app-pools/${poolId}/${cmd}`),

  syncPools: (projectKey: string, appId: number, poolIds: number[]) =>
    api.post(`/projects/${projectKey}/applications/${appId}/app-pools`, {
      app_pool_ids: poolIds,
    }),

  removePool: (projectKey: string, appId: number, poolId: number) =>
    api.delete(`/projects/${projectKey}/applications/${appId}/app-pools/${poolId}`),

  updatePoolLogPath: (projectKey: string, appId: number, poolId: number, logPath: string) =>
    api.put(`/projects/${projectKey}/applications/${appId}/app-pools/${poolId}`, {
      log_path: logPath,
    }),

  delete: (projectKey: string, appId: number) =>
    api.delete(`/projects/${projectKey}/applications/${appId}`),
}
