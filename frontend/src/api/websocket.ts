import { useAuthStore } from '../stores/authStore'

type MessageHandler = (data: unknown) => void

interface WsMessage {
  action: string
  channel?: string
  data?: unknown
}

class WebSocketClient {
  private ws: WebSocket | null = null
  private handlers: Map<string, Set<MessageHandler>> = new Map()
  private reconnectAttempts = 0
  private maxReconnectAttempts = 5
  private reconnectDelay = 1000
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private url: string = ''

  private static instance: WebSocketClient

  private constructor() {}

  static getInstance(): WebSocketClient {
    if (!WebSocketClient.instance) {
      WebSocketClient.instance = new WebSocketClient()
    }
    return WebSocketClient.instance
  }

  connect(): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      return
    }

    const token = useAuthStore.getState().token
    if (!token) {
      console.warn('WebSocket connect: no auth token available')
      return
    }

    // Build WebSocket URL
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    this.url = `${protocol}//${host}/ws?token=${encodeURIComponent(token)}`

    this.createConnection()
  }

  private createConnection(): void {
    this.ws = new WebSocket(this.url)

    this.ws.onopen = () => {
      console.log('WebSocket connected')
      this.reconnectAttempts = 0
      this.reconnectDelay = 1000

      // Subscribe to default channels
      this.subscribe('log')
      this.subscribe('alert')
      this.subscribe('device_event')
    }

    this.ws.onmessage = (event) => {
      try {
        const message: WsMessage = JSON.parse(event.data)
        this.dispatchMessage(message)
      } catch (err) {
        console.error('WebSocket failed to parse message:', err)
      }
    }

    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error)
    }

    this.ws.onclose = () => {
      console.log('WebSocket disconnected')
      this.ws = null
      this.scheduleReconnect()
    }
  }

  private scheduleReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.warn('WebSocket: max reconnect attempts reached')
      return
    }

    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
    }

    const delay = this.reconnectDelay
    this.reconnectAttempts++
    this.reconnectDelay = Math.min(this.reconnectDelay * 2, 30000)

    console.log(`WebSocket: reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`)

    this.reconnectTimer = setTimeout(() => {
      this.connect()
    }, delay)
  }

  disconnect(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }

    this.reconnectAttempts = this.maxReconnectAttempts // Prevent auto-reconnect

    if (this.ws) {
      this.ws.close()
      this.ws = null
    }
  }

  subscribe(channel: string): void {
    this.sendMessage({ action: 'subscribe', channel })
  }

  unsubscribe(channel: string): void {
    this.sendMessage({ action: 'unsubscribe', channel })
  }

  on(channel: string, handler: MessageHandler): void {
    if (!this.handlers.has(channel)) {
      this.handlers.set(channel, new Set())
    }
    this.handlers.get(channel)!.add(handler)
  }

  off(channel: string, handler: MessageHandler): void {
    const channelHandlers = this.handlers.get(channel)
    if (channelHandlers) {
      channelHandlers.delete(handler)
      if (channelHandlers.size === 0) {
        this.handlers.delete(channel)
      }
    }
  }

  private sendMessage(message: WsMessage): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message))
    }
  }

  private dispatchMessage(message: WsMessage): void {
    const { channel, data } = message
    if (!channel) return

    const channelHandlers = this.handlers.get(channel)
    if (channelHandlers) {
      channelHandlers.forEach((handler) => {
        try {
          handler(data)
        } catch (err) {
          console.error(`WebSocket handler error for channel ${channel}:`, err)
        }
      })
    }
  }
}

export const wsClient = WebSocketClient.getInstance()