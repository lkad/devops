import { useAuthStore } from '../stores/authStore'

export class ApiError extends Error {
  status: number
  data: unknown

  constructor(message: string, status: number, data?: unknown) {
    super(message)
    this.status = status
    this.data = data
    this.name = 'ApiError'
  }
}

type RequestOptions = {
  headers?: Record<string, string>
  params?: Record<string, string>
}

async function request<T>(
  method: string,
  url: string,
  body?: unknown,
  options: RequestOptions = {}
): Promise<T> {
  const { headers = {}, params } = options

  let queryString = ''
  if (params) {
    const searchParams = new URLSearchParams(params)
    queryString = searchParams.toString() ? `?${searchParams.toString()}` : ''
  }

  const fullUrl = `${url}${queryString}`

  const token = useAuthStore.getState().token
  const authHeaders: Record<string, string> = {}
  if (token) {
    authHeaders['Authorization'] = `Bearer ${token}`
  }

  const response = await fetch(fullUrl, {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...authHeaders,
      ...headers,
    },
    body: body ? JSON.stringify(body) : undefined,
  })

  if (response.status === 401) {
    useAuthStore.getState().logout()
    window.location.href = '/login'
    throw new ApiError('Unauthorized', 401)
  }

  if (!response.ok) {
    let errorData: unknown
    try {
      errorData = await response.json()
    } catch {
      errorData = await response.text()
    }
    throw new ApiError(
      `Request failed with status ${response.status}`,
      response.status,
      errorData
    )
  }

  if (response.status === 204) {
    return undefined as T
  }

  return response.json()
}

export const apiClient = {
  get: <T>(url: string, options?: RequestOptions) =>
    request<T>('GET', url, undefined, options),

  post: <T>(url: string, body: unknown, options?: RequestOptions) =>
    request<T>('POST', url, body, options),

  put: <T>(url: string, body: unknown, options?: RequestOptions) =>
    request<T>('PUT', url, body, options),

  delete: <T>(url: string, options?: RequestOptions) =>
    request<T>('DELETE', url, undefined, options),
}