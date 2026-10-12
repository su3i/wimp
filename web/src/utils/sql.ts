import type { SqlInstance, SqlInstanceWithMachine } from '@/types'

export function instanceLabel(inst: Pick<SqlInstance, 'Name'>) {
  return inst.Name === 'MSSQLSERVER' ? 'MSSQLSERVER (default)' : inst.Name
}

export function apiError(err: unknown, fallback: string) {
  return (err as { response?: { data?: { error?: string } } })?.response?.data?.error ?? fallback
}

// One SQL Server instance as people think of it. The API returns one row per host that
// reports it, so a failover cluster instance arrives as several rows; this folds them into one.
export interface SqlInstanceGroup {
  key: string
  // The row to show and act on: the node running it, when there is one.
  current: SqlInstanceWithMachine
  rows: SqlInstanceWithMachine[]
  // How it is connected to: SERVER\INSTANCE, or just SERVER for a default instance.
  name: string
  server: string
}

function groupKey(i: SqlInstanceWithMachine) {
  return i.Clustered && i.VirtualServerName ? `fci:${i.VirtualServerName}/${i.Name}` : `${i.MachineID}:${i.Name}`
}

function pickCurrent(rows: SqlInstanceWithMachine[]) {
  const online = rows.filter(r => r.machine.Status === 'online')
  return (
    online.find(r => !r.Clustered || r.OwnerNode.toLowerCase() === r.machine.Hostname?.toLowerCase()) ??
    online[0] ??
    rows[0]
  )
}

export function groupInstances(rows: SqlInstanceWithMachine[]): SqlInstanceGroup[] {
  const byKey = new Map<string, SqlInstanceWithMachine[]>()
  for (const r of rows) byKey.set(groupKey(r), [...(byKey.get(groupKey(r)) ?? []), r])
  return [...byKey.entries()]
    .map(([key, rs]) => {
      const current = pickCurrent(rs)
      const server = (current.Clustered && current.VirtualServerName ? current.VirtualServerName : current.machine.Hostname ?? '').toUpperCase()
      const name = current.Name === 'MSSQLSERVER' ? server : `${server}\\${current.Name}`
      return { key, current, rows: rs, name, server }
    })
    .sort((a, b) => a.name.localeCompare(b.name))
}

// The group a row belongs to, so a URL holding any of its row ids keeps working after a failover.
export function findGroup(groups: SqlInstanceGroup[], rowId: number) {
  return groups.find(g => g.rows.some(r => r.ID === rowId))
}

// A clustered instance's metrics come from whichever node runs it, so queries match every node
// it is installed on and the series stays continuous across a failover.
export function promSelector(group: SqlInstanceGroup, instanceLabel: 'mssql_instance' | 'sql_instance') {
  const hosts = [...new Set(group.rows.map(r => (r.machine.Hostname ?? '').replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))]
  const name = group.current.Name.replace(/\\/g, '\\\\').replace(/"/g, '\\"')
  return `hostname=~"(?i)(${hosts.join('|')})",${instanceLabel}="${name}"`
}

export function fmtDuration(seconds: number) {
  if (seconds < 60) return `${Math.round(seconds)}s`
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`
  if (seconds < 48 * 3600) return `${(seconds / 3600).toFixed(1)}h`
  return `${Math.floor(seconds / 86400)}d`
}

export function fmtBytes(bytes: number) {
  if (bytes < 1024 ** 2) return `${(bytes / 1024).toFixed(0)} KB`
  if (bytes < 1024 ** 3) return `${(bytes / 1024 ** 2).toFixed(0)} MB`
  if (bytes < 1024 ** 4) return `${(bytes / 1024 ** 3).toFixed(1)} GB`
  return `${(bytes / 1024 ** 4).toFixed(2)} TB`
}

// SQL Server reports its major version; people know releases by year.
const RELEASES: Record<string, string> = {
  '11': '2012', '12': '2014', '13': '2016', '14': '2017', '15': '2019', '16': '2022', '17': '2025',
}

export function sqlRelease(version: string, edition: string) {
  const year = RELEASES[version.split('.')[0]]
  const ed = edition.replace(/ Edition.*$/, '').replace(/\s*\(64-bit\)/, '')
  return [year ? `SQL Server ${year}` : 'SQL Server', ed].filter(Boolean).join(' ')
}

// One status for an instance, folding SQL Agent into it: an engine that runs without its
// Agent is up, but its scheduled jobs (backups included) are not running.
export function instanceStatus(inst: Pick<SqlInstance, 'State' | 'AgentState'>, hostOnline: boolean) {
  if (!hostOnline) return { label: 'Unknown', tone: 'muted' as const, detail: 'The host reporting this instance is offline.' }
  if (inst.State !== 'Running') {
    const down = inst.State === 'Stopped' || inst.State === 'Failed'
    return { label: inst.State, tone: down ? ('danger' as const) : ('warning' as const), detail: undefined }
  }
  if (inst.AgentState !== 'Running') {
    return {
      label: 'Agent stopped',
      tone: 'warning' as const,
      detail: `SQL Server is running, but SQL Agent is ${inst.AgentState.toLowerCase()}: scheduled jobs, backups included, are not running.`,
    }
  }
  return { label: 'Running', tone: 'success' as const, detail: undefined }
}
