import { api } from "@/lib/axios";

export type Level = "info" | "warning" | "critical" | "sev" | "disabled";

export interface AlertSeverity {
  alert_type: string;
  category: string;
  level: Level;
  default: Level;
  overridden: boolean;
}

export interface ToggleSetting {
  value: boolean;
  default: boolean;
  overridden: boolean;
}

export interface LevelSetting {
  value: Level;
  default: Level;
  overridden: boolean;
}

export interface AgentSettings {
  version: string;
  auto_update: boolean;
  release_base: string;
}

export interface ServiceStatus {
  name: string;
  configured: boolean;
  reachable: boolean;
  detail: string;
}

export interface Settings {
  alerting_enabled: ToggleSetting;
  receiver_min_severity: LevelSetting;
  enforce_mfa: ToggleSetting;
  alert_severities: AlertSeverity[];
  agent: AgentSettings;
  services: ServiceStatus[];
}

// A null value resets the key to its deployment default, which is a different outcome
// from setting it to an empty string.
export interface SettingChange {
  key: string;
  value: string | null;
}

export const settingService = {
  get: async (): Promise<{ settings: Settings; levels: Level[] }> => {
    const { data } = await api.get<{ settings: Settings; levels: Level[] }>("/settings");
    return data;
  },

  update: async (changes: SettingChange[]): Promise<{ settings: Settings }> => {
    const { data } = await api.put<{ settings: Settings }>("/settings", { changes });
    return data;
  },
};

// Mirrors the writable keys in internal/domain/setting/keys.go. security.enforce_mfa is
// intentionally absent - it is read-only until account MFA enrolment exists.
export const SETTING_KEYS = {
  alertingEnabled: "alerting.enabled",
  receiverMinSeverity: "alerting.receiver_min_severity",
  severity: (alertType: string) => `alert.severity.${alertType}`,
};
