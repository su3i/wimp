import { api } from "@/lib/axios";

// ── Types ─────────────────────────────────────────────────────────────────────

export interface DashboardStats {
  machines_count: number;
  applications_count: number;
  unread_notifications: number;
}

export interface ActiveAlert {
  id: string;
  category: "machine" | "app_pool" | "service" | string;
  message: string;
  fired_at: string;
}

export interface UptimeRanking {
  name: string;
  uptime: number; // 0–100
}

export interface HeatmapDay {
  date: string; // YYYY-MM-DD
  status: "ok" | "warning" | "critical" | "no_data";
  incident_count: number;
  downtime_minutes: number;
}

export interface HeatmapMachine {
  machine_id: number;
  hostname: string;
  days: HeatmapDay[];
}

export interface UptimeData {
  platform_availability: number | null;
  best: UptimeRanking[];
  worst: UptimeRanking[];
  heatmap: HeatmapMachine[];
}

export interface DashboardDisk {
  label: string;
  used_percent: number;
}

export interface DashboardMachine {
  id: number;
  hostname: string;
  status: "online" | "offline" | "pending";
  last_seen_at: string | null;
  cpu_percent: number;
  ram_used_gb: number;
  ram_total_gb: number;
  disks: DashboardDisk[];
  iis_running: boolean;
  app_pools_running: number;
  app_pools_total: number;
}

export interface DashboardNotification {
  ID: number;
  CreatedAt: string;
  Level: "info" | "warning" | "critical" | string;
  Category: "machine" | "app_pool" | "service" | string;
  Title: string;
  Detail: string;
  MachineID: number | null;
  ReadAt: string | null;
}

export interface AlertHistoryPoint {
  hour: string; // ISO timestamp - start of the hour
  count: number;
}

// ── Service ───────────────────────────────────────────────────────────────────

export const dashboardService = {
  async getStats(projectKey: string): Promise<DashboardStats | null> {
    try {
      const { data } = await api.get(`/projects/${projectKey}/dashboard/stats`);
      return data as DashboardStats;
    } catch {
      return null;
    }
  },

  async getActiveAlerts(): Promise<ActiveAlert[]> {
    try {
      const { data } = await api.get("/dashboard/alerts/active");
      return (data.alerts as ActiveAlert[]) ?? [];
    } catch {
      return [];
    }
  },

  async getUptime(days = 30): Promise<UptimeData | null> {
    try {
      const { data } = await api.get("/dashboard/uptime", { params: { days } });
      return {
        platform_availability: data.platform_availability ?? null,
        best: data.best ?? [],
        worst: data.worst ?? [],
        heatmap: data.heatmap ?? [],
      };
    } catch {
      return null;
    }
  },

  async getMachines(): Promise<DashboardMachine[]> {
    try {
      const { data } = await api.get("/dashboard/machines");
      return (data.machines as DashboardMachine[]) ?? [];
    } catch {
      return [];
    }
  },

  async getAlertHistory(hours = 24): Promise<AlertHistoryPoint[]> {
    try {
      const { data } = await api.get("/dashboard/alert-history", { params: { hours } });
      return (data.history as AlertHistoryPoint[]) ?? [];
    } catch {
      return [];
    }
  },

  async getNotifications(projectKey?: string): Promise<DashboardNotification[]> {
    try {
      const url = projectKey ? `/projects/${projectKey}/notifications` : "/notifications";
      const { data } = await api.get(url, { params: { limit: 10 } });
      return (data.notifications as DashboardNotification[]) ?? [];
    } catch {
      return [];
    }
  },

  async listNotifications(
    projectKey: string,
    params: { page: number; limit: number; level?: string },
  ): Promise<{ notifications: DashboardNotification[]; total: number }> {
    try {
      const { data } = await api.get(`/projects/${projectKey}/notifications`, { params });
      return {
        notifications: (data.notifications as DashboardNotification[]) ?? [],
        total: (data.total as number) ?? 0,
      };
    } catch {
      return { notifications: [], total: 0 };
    }
  },
};
