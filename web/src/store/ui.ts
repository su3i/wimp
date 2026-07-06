import { create } from 'zustand'

interface UIState {
  sidebarExpanded: boolean
  setSidebarExpanded: (v: boolean) => void
}

export const useUIStore = create<UIState>((set) => ({
  sidebarExpanded: localStorage.getItem('wimp_sidebar_expanded') === 'true',
  setSidebarExpanded: (v) => set({ sidebarExpanded: v }),
}))
