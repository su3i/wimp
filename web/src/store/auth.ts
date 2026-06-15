import { create } from 'zustand'

export interface AuthUser {
  id: string
  email: string
  roles: string[]
}

interface AuthStore {
  accessToken: string | null
  refreshToken: string | null
  user: AuthUser | null
  setAuth: (accessToken: string, refreshToken: string) => void
  clearAuth: () => void
}

function parseJwt(token: string): AuthUser {
  try {
    const b64 = token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')
    const payload = JSON.parse(atob(b64))
    return {
      id: String(payload.sub ?? ''),
      email: payload.email ?? '',
      roles: Array.isArray(payload.roles) ? payload.roles : [],
    }
  } catch {
    return { id: '', email: '', roles: [] }
  }
}

function loadFromStorage(): Pick<AuthStore, 'accessToken' | 'refreshToken' | 'user'> {
  try {
    const accessToken = localStorage.getItem('wimp_at')
    const refreshToken = localStorage.getItem('wimp_rt')
    if (accessToken) {
      return { accessToken, refreshToken, user: parseJwt(accessToken) }
    }
  } catch { /* */ }
  return { accessToken: null, refreshToken: null, user: null }
}

export const useAuthStore = create<AuthStore>(set => ({
  ...loadFromStorage(),

  setAuth: (accessToken, refreshToken) => {
    localStorage.setItem('wimp_at', accessToken)
    if (refreshToken) localStorage.setItem('wimp_rt', refreshToken)
    set({ accessToken, refreshToken, user: parseJwt(accessToken) })
  },

  clearAuth: () => {
    localStorage.removeItem('wimp_at')
    localStorage.removeItem('wimp_rt')
    set({ accessToken: null, refreshToken: null, user: null })
  },
}))
