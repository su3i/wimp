import { api } from '@/lib/axios'
import type { Project } from '@/types'

interface CreateProjectPayload {
  name: string
  key: string
  businessDomain?: string
  orgKey?: string
}

interface UpdateProjectPayload {
  name?: string
  businessDomain?: string
  status?: string
}

export const projectService = {
  list: () =>
    api.get<{ message: string; projects: Project[] }>('/projects'),

  get: (key: string) =>
    api.get<{ message: string; project: Project }>(`/project/${key}`),

  create: (payload: CreateProjectPayload) =>
    api.post<{ message: string; project: Project }>('/project', payload),

  update: (key: string, payload: UpdateProjectPayload) =>
    api.put<Project>(`/project/${key}`, payload),

  delete: (key: string) =>
    api.delete(`/project/${key}`),
}
