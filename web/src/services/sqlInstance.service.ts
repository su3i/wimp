import { api } from '@/lib/axios'
import type { SqlInstance, SqlInstanceWithMachine, SqlLiveKind } from '@/types'

type InstanceCommand = 'start' | 'restart'

export const sqlInstanceService = {
  list: (projectKey: string, machineId: number) =>
    api.get<{ message: string; sql_instances: SqlInstance[] }>(
      `/projects/${projectKey}/machines/${machineId}/sql-instances`,
    ),

  listProject: (projectKey: string) =>
    api.get<{ message: string; sql_instances: SqlInstanceWithMachine[] }>(`/projects/${projectKey}/sql-instances`),

  live: (projectKey: string, machineId: number, instance: string, kind: SqlLiveKind) =>
    api.get<{ message: string; rows: Record<string, unknown>[] | null }>(
      `/projects/${projectKey}/machines/${machineId}/sql-instances/${encodeURIComponent(instance)}/live/${kind}`,
    ),

  command: (projectKey: string, machineId: number, instance: string, cmd: InstanceCommand) =>
    api.post(`/projects/${projectKey}/machines/${machineId}/sql-instances/${encodeURIComponent(instance)}/${cmd}`),

  move: (projectKey: string, machineId: number, instance: string, node: string) =>
    api.post(`/projects/${projectKey}/machines/${machineId}/sql-instances/${encodeURIComponent(instance)}/move`, { node }),

  failoverAg: (projectKey: string, machineId: number, ag: string, node: string) =>
    api.post(`/projects/${projectKey}/machines/${machineId}/availability-groups/${encodeURIComponent(ag)}/failover`, { node }),

  addToApplication: (projectKey: string, appId: number, ids: number[]) =>
    api.post(`/projects/${projectKey}/applications/${appId}/sql-instances`, { sql_instance_ids: ids }),

  removeFromApplication: (projectKey: string, appId: number, instanceId: number) =>
    api.delete(`/projects/${projectKey}/applications/${appId}/sql-instances/${instanceId}`),
}
