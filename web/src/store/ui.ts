import { create } from 'zustand'

interface UIState {
  sidebarExpanded: boolean
  setSidebarExpanded: (v: boolean) => void

  // Project key the Dashboard's add-a-host hint was dismissed for. Held in memory rather
  // than localStorage on purpose: dismissing it lasts for the session, while switching to
  // another project and back shows it again (the key no longer matches).
  emptyProjectPrompted: string | null
  markEmptyProjectPrompted: (projectKey: string) => void
}

export const useUIStore = create<UIState>((set) => ({
  sidebarExpanded: localStorage.getItem('wimp_sidebar_expanded') === 'true',
  setSidebarExpanded: (v) => set({ sidebarExpanded: v }),

  emptyProjectPrompted: null,
  markEmptyProjectPrompted: (projectKey) => set({ emptyProjectPrompted: projectKey }),
}))
