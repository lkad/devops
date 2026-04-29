import { useEffect, useCallback, useRef, useState } from 'react'
import { useAuthStore } from '../stores/authStore'

export interface WebSocketMessage {
  channel: string
  type: string
  data: unknown
  timestamp: string
}

export interface UseWebSocketOptions {
  channels?: string[]
  onMessage?: (message: WebSocketMessage) => void
  onConnect?: () => void
  onDisconnect?: () => void
  reconnectAttempts?: number
}

type MessageHandler = (data: unknown) => void

interface UseWebSocketReturn {
  isConnected: boolean
  send: (message: { action: string; channel?: string; data?: unknown }) => void
  on: (channel: string, handler: MessageHandler) => void
  off: (channel: string, handler: MessageHandler) => void
}

export function useWebSocket({
  channels,
  onMessage,
  onConnect,
  onDisconnect,
  reconnectAttempts = 5,
}: UseWebSocketOptions = {}): UseWebSocketReturn {
  const [isConnected, setIsConnected] = useState(false)
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const reconnectCountRef = useRef(0)
  const pingIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const handlersRef = useRef<Map<string, Set<MessageHandler>>>(new Map())

  const token = useAuthStore((state) => state.token)

  const clearTimers = useCallback(() => {
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current)
      reconnectTimerRef.current = null
    }
    if (pingIntervalRef.current) {
      clearInterval(pingIntervalRef.current)
      pingIntervalRef.current = null
    }
  }, [])

  const connect = useCallback(() => {
    if (!token) {
      console.warn('WebSocket: no auth token available')
      return
    }

    if (wsRef.current?.readyState === WebSocket.OPEN) {
      return
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    const url = `${protocol}//${host}/ws?token=${encodeURIComponent(token)}`

    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onopen = () => {
      console.log('WebSocket connected')
      setIsConnected(true)
      reconnectCountRef.current = 0
      onConnect?.()

      // Subscribe to requested channels
      ;(channels || []).forEach((channel) => {
        ws.send(JSON.stringify({ action: 'subscribe', channel }))
      })

      // Start ping interval for keepalive
      pingIntervalRef.current = setInterval(() => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ action: 'ping' }))
        }
      }, 30000)
    }

    ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data) as WebSocketMessage
        // Ignore pong responses
        if (message.type === 'pong') return

        // Call onMessage callback if provided
        onMessage?.(message)

        // Call channel-specific handlers
        const channelHandlers = handlersRef.current.get(message.channel)
        if (channelHandlers) {
          channelHandlers.forEach((handler) => handler(message.data))
        }
      } catch (err) {
        console.error('WebSocket: failed to parse message:', err)
      }
    }

    ws.onerror = (error) => {
      console.error('WebSocket error:', error)
    }

    ws.onclose = () => {
      console.log('WebSocket disconnected')
      setIsConnected(false)
      clearTimers()
      onDisconnect?.()

      // Schedule reconnect with exponential backoff
      if (reconnectCountRef.current < reconnectAttempts) {
        const delay = Math.min(1000 * Math.pow(2, reconnectCountRef.current), 30000)
        reconnectCountRef.current++
        console.log(`WebSocket: reconnecting in ${delay}ms (attempt ${reconnectCountRef.current})`)
        reconnectTimerRef.current = setTimeout(connect, delay)
      } else {
        console.warn('WebSocket: max reconnect attempts reached')
      }
    }
  }, [token, channels, onMessage, onConnect, onDisconnect, reconnectAttempts, clearTimers])

  useEffect(() => {
    connect()

    return () => {
      clearTimers()
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
    }
  }, [connect, clearTimers])

  const send = useCallback((message: { action: string; channel?: string; data?: unknown }) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(message))
    } else {
      console.warn('WebSocket: cannot send, not connected')
    }
  }, [])

  const on = useCallback((channel: string, handler: MessageHandler) => {
    if (!handlersRef.current.has(channel)) {
      handlersRef.current.set(channel, new Set())
    }
    handlersRef.current.get(channel)!.add(handler)
  }, [])

  const off = useCallback((channel: string, handler: MessageHandler) => {
    const channelHandlers = handlersRef.current.get(channel)
    if (channelHandlers) {
      channelHandlers.delete(handler)
    }
  }, [])

  return { isConnected, send, on, off }
}

// Hook for simple authentication state access
export function useAuth() {
  const token = useAuthStore((state) => state.token)
  const user = useAuthStore((state) => state.user)
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated)

  return { user, isAuthenticated, token }
}

// Hook that redirects to login if not authenticated
export function useRequireAuth() {
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated)
  return isAuthenticated
}