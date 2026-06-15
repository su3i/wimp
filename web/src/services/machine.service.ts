import { api } from '@/lib/axios'
import type { MachineWithPools } from '@/types'

export const machineService = {
  list: (projectKey: string) =>
    api.get<{ message: string; machines: MachineWithPools[] }>(`/projects/${projectKey}/machines`),

  create: (projectKey: string) =>
    api.post<{ message: string; machine: MachineWithPools; download_command: string; run_command: string }>(
      `/projects/${projectKey}/machines`,
    ),

  delete: (projectKey: string, machineId: number) =>
    api.delete<{ message: string; uninstall_command: string }>(`/projects/${projectKey}/machines/${machineId}`),
}
