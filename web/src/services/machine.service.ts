import { api } from '@/lib/axios'
import type { MachineWithPools } from '@/types'

type MachineCommand = 'shutdown' | 'restart'

export const machineService = {
  list: (projectKey: string, params?: { page?: number; per_page?: number; status?: string }) =>
    api.get<{ message: string; machines: MachineWithPools[]; total: number; page: number; per_page: number }>(
      `/projects/${projectKey}/machines`,
      { params },
    ),

  create: (projectKey: string) =>
    api.post<{ message: string; machine: MachineWithPools; download_command: string; run_command: string }>(
      `/projects/${projectKey}/machines`,
    ),

  delete: (projectKey: string, machineId: number) =>
    api.delete<{ message: string; uninstall_command: string }>(`/projects/${projectKey}/machines/${machineId}`),

  command: (projectKey: string, machineId: number, cmd: MachineCommand) =>
    api.post(`/projects/${projectKey}/machines/${machineId}/${cmd}`),
}
