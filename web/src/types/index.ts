// ── Auth ──────────────────────────────────────────────────────────────────

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

// ── Account ───────────────────────────────────────────────────────────────

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

// ── Organization ──────────────────────────────────────────────────────────

export type OrgScope = 'PUBLIC' | 'PRIVATE'

export interface Organization {
  ID: number
  Name: string
  Key: string
  Scope: OrgScope
  CreatedAt: string
  UpdatedAt: string
}

// ── Project ───────────────────────────────────────────────────────────────

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

// ── Machine ───────────────────────────────────────────────────────────────

export type MachineStatus = 'pending' | 'online' | 'offline'

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
  CreatedAt: string
  UpdatedAt: string
}

export interface MachineWithPools extends Machine {
  app_pools: AppPool[]
}

// ── App Pool ──────────────────────────────────────────────────────────────

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

// ── Site ──────────────────────────────────────────────────────────────────

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

// ── Application ───────────────────────────────────────────────────────────

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

// ── Loki ──────────────────────────────────────────────────────────────────

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


// ── Monitor ───────────────────────────────────────────────────────────────

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
