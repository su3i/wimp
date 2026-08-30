import { api } from "@/lib/axios";
import type { Incident, IncidentStatus } from "@/types";

export interface IncidentsResponse {
  incidents: Incident[];
  total: number;
  page: number;
  per_page: number;
  counts: { open: number; resolved: number };
}

export const incidentService = {
  list: async (
    projectKey: string,
    params: { page?: number; per_page?: number; status?: IncidentStatus },
  ): Promise<IncidentsResponse> => {
    const { data } = await api.get<IncidentsResponse>(`/projects/${projectKey}/incidents`, { params });
    return data;
  },
};
