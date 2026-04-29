import { apiClient } from '../client'
import type { User } from '../../stores/authStore'

export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  token: string
  user: User
}

export const authApi = {
  login: (data: LoginRequest) =>
    apiClient.post<LoginResponse>('/api/auth/login', data),

  logout: () =>
    apiClient.post<void>('/api/auth/logout', undefined),

  me: () =>
    apiClient.get<User>('/api/auth/me'),
}