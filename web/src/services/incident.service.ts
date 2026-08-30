import { api } from "@/lib/axios";
import type { Incident } from "@/types";

export interface IncidentsResponse {
  incidents: Incident[];
  total: number;
  page: number;
  per_page: number;
  // How far back the feed reaches, so the end-of-list notice can name the window rather
  // than hardcoding a number the server owns.
  window_days: number;
  counts: { open: number; resolved: number };
}

export const incidentService = {
  list: async (
    projectKey: string,
    params: { page?: number; per_page?: number },
  ): Promise<IncidentsResponse> => {
    const { data } = await api.get<IncidentsResponse>(`/projects/${projectKey}/incidents`, { params });
    return data;
  },

  resolve: (projectKey: string, incidentId: number) =>
    api.post(`/projects/${projectKey}/incidents/${incidentId}/resolve`),
};
