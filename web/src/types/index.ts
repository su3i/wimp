export interface LoginResponse {
  message: string
  access_token: string
  refresh_token: string
}

export interface LoginMfaResponse {
  message: string
  mfa_required: true
  challenge_id: string
}

export interface TokenPayload {
  sub: string
  username: string
  roles: string[]
  exp: number
  iat: number
}

export type AccountRole = 'SUPERADMIN' | 'ADMIN' | 'GUEST'

export interface AccountDTO {
  ID: number
  Name: string
  Username: string
  Role: AccountRole
  MFAEnabled: boolean
  CreatedAt: string
  UpdatedAt: string
}

export type SecurityLevel = 'Excellent' | 'Strong' | 'Fair' | 'Weak'

export type OrgScope = 'PUBLIC' | 'PRIVATE'

export interface Organization {
  ID: number
  Name: string
  Key: string
  Scope: OrgScope
  CreatedAt: string
  UpdatedAt: string
}

export type ProjectStatus = 'ACTIVE' | 'PAUSED' | 'ARCHIVED'

export interface Project {
  ID: number
  OrganizationID: number
  Name: string
  Key: string
  Status: ProjectStatus
  BusinessDomain: string
  CreatedBy: Record<string, string>
  CreatedAt: string
  UpdatedAt: string
}

// 'reassigned' is terminal: the physical machine was re-bootstrapped and now reports to a
// different host entry (usually in another project), so this one will never come back.
export type MachineStatus = 'pending' | 'online' | 'offline' | 'reassigned'

export interface Machine {
  ID: number
  ProjectID: number
  Hostname: string
  IPs: string[]
  Status: MachineStatus
  Token: string
  TokenExpiresAt: string
  LastSeenAt: string | null
  AgentVersion: string
  WindowsVersion: string
  MachineUID: string
  SupersededByID: number | null
  ReassignedAt: string | null
  CreatedAt: string
  UpdatedAt: string
}

export interface MachineWithPools extends Machine {
  app_pools: AppPool[]
}

export interface AppPool {
  ID: number
  MachineID: number
  Name: string
  State: string
  RuntimeVersion: string
  PipelineMode: string
  StartMode: string
  IdentityType: string
  CreatedAt: string
  UpdatedAt: string
}

export interface Binding {
  protocol: string
  ip: string
  port: string
  hostname: string
}

export interface Site {
  ID: number
  MachineID: number
  Name: string
  State: string
  PhysicalPath: string
  AppPoolName: string
  Bindings: Binding[]
  CreatedAt: string
  UpdatedAt: string
}

export interface Application {
  ID: number
  ProjectID: number
  Name: string
  HealthCheckURL?: string | null
  HealthCheckIntervalSeconds?: number
  ConsecutiveFailures?: number
  AlertFired?: boolean
  CreatedAt: string
  UpdatedAt: string
  pool_total?: number
  pool_healthy?: number
}

export interface AppPoolWithDetails extends AppPool {
  machine: Machine
  sites: Site[]
  log_path?: string | null
}

export interface ApplicationDetail extends Application {
  app_pools: AppPoolWithDetails[]
}

export interface LokiStream {
  stream: Record<string, string>
  values: [string, string][] // [nanosecond timestamp, log line]
}

export interface LokiResponse {
  status: string
  data: {
    resultType: string
    result: LokiStream[]
  }
}

export interface Monitor {
  ID: number
  ProjectID: number
  Name: string
  URL: string
  IntervalSeconds: number
  Enabled: boolean
  CreatedAt: string
  UpdatedAt: string
}

export type IncidentStatus = 'open' | 'resolved'

// An incident is the span between an alert that reports a fault and the alert that reports
// it recovered. Both ends are denormalized onto the row, so the timeline renders without
// joining back to the notifications the incident was assembled from.
export interface Incident {
  ID: number
  ProjectID: number
  MachineID: number
  Kind: string
  Subject: string
  Instance: string
  Level: 'info' | 'warning' | 'critical' | 'sev'
  Category: string
  Status: IncidentStatus
  StartedAt: string
  ResolvedAt: string | null
  OpenedTitle: string
  OpenedDetail: string
  ResolvedTitle: string
  ResolvedDetail: string
  CreatedAt: string
  UpdatedAt: string
}
