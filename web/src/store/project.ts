import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { Project } from '@/types'

interface ProjectStore {
  activeProject: Project | null
  setActiveProject: (project: Project) => void
  clearActiveProject: () => void
}

export const useProjectStore = create<ProjectStore>()(
  persist(
    set => ({
      activeProject: null,
      setActiveProject: project => set({ activeProject: project }),
      clearActiveProject: () => set({ activeProject: null }),
    }),
    { name: 'wimp_project' },
  ),
)
