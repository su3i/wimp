import { api } from '@/lib/axios'
import type { MachineWithPools, OsType } from '@/types'

export const machineService = {
  list: (projectKey: string, params?: { page?: number; per_page?: number; status?: string }) =>
    api.get<{ message: string; machines: MachineWithPools[]; total: number; page: number; per_page: number }>(
      `/projects/${projectKey}/machines`,
      { params },
    ),

  create: (projectKey: string, osType: OsType) =>
    api.post<{ message: string; machine: MachineWithPools; download_command: string; run_command: string }>(
      `/projects/${projectKey}/machines`,
      { os_type: osType },
    ),

  delete: (projectKey: string, machineId: number) =>
    api.delete<{ message: string; uninstall_command: string }>(`/projects/${projectKey}/machines/${machineId}`),

  restart: (projectKey: string, machineId: number) =>
    api.post(`/projects/${projectKey}/machines/${machineId}/restart`),
}
