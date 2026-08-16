import { create } from 'zustand'

interface UIState {
  sidebarExpanded: boolean
  setSidebarExpanded: (v: boolean) => void

  // Project key the Dashboard's get-started modal has already been shown for. Held in
  // memory rather than localStorage on purpose: the prompt should fire once per project
  // per session, so navigating Overview -> Hosts -> Overview stays quiet, while switching
  // to another project and back prompts again (the key no longer matches).
  emptyProjectPrompted: string | null
  markEmptyProjectPrompted: (projectKey: string) => void
}

export const useUIStore = create<UIState>((set) => ({
  sidebarExpanded: localStorage.getItem('wimp_sidebar_expanded') === 'true',
  setSidebarExpanded: (v) => set({ sidebarExpanded: v }),

  emptyProjectPrompted: null,
  markEmptyProjectPrompted: (projectKey) => set({ emptyProjectPrompted: projectKey }),
}))
