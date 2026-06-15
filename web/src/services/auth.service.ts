import { api } from '@/lib/axios'
import type { LoginResponse, LoginMfaResponse } from '@/types'

export const authService = {
  login: (email: string, password: string) =>
    api.post<LoginResponse | LoginMfaResponse>('/auth/login', { email, password }),

  mfa: (challengeId: string, code: string) =>
    api.post<LoginResponse>('/auth/mfa', { challenge_id: challengeId, code }),

  logout: (refreshToken: string) =>
    api.post('/auth/revoke-token', { refresh_token: refreshToken }),
}
