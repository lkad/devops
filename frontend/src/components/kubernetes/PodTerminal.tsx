import { useEffect, useRef, useCallback } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { useAuthStore } from '../../stores/authStore'
import styles from './PodTerminal.module.css'

interface PodTerminalProps {
  podName: string
  containerName?: string
  namespace?: string
  onClose?: () => void
}

export function PodTerminal({ podName, containerName, namespace = 'default', onClose }: PodTerminalProps) {
  const terminalRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const resizeObserverRef = useRef<ResizeObserver | null>(null)

  const token = useAuthStore((state) => state.token)

  const handleData = useCallback((data: string) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ action: 'stdin', data }))
    }
  }, [])

  const handleResize = useCallback(({ rows, cols }: { rows: number; cols: number }) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ action: 'resize', rows, cols }))
    }
  }, [])

  useEffect(() => {
    if (!terminalRef.current) return

    // Initialize terminal
    const term = new Terminal({
      cursorBlink: true,
      fontFamily: 'JetBrains Mono, monospace',
      fontSize: 14,
      theme: {
        background: '#0c1220',
        foreground: '#f1f5f9',
        cursor: '#22d3ee',
        selectionBackground: 'rgba(34, 211, 238, 0.3)',
      },
      scrollback: 1000,
    })

    const fitAddon = new FitAddon()
    term.loadAddon(fitAddon)
    term.open(terminalRef.current)
    fitAddon.fit()

    termRef.current = term
    fitAddonRef.current = fitAddon

    // Connect to WebSocket exec endpoint
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    const execUrl = `${protocol}//${host}/api/k8s/exec?pod=${encodeURIComponent(podName)}&container=${encodeURIComponent(containerName || '')}&namespace=${encodeURIComponent(namespace)}&token=${encodeURIComponent(token || '')}`

    const ws = new WebSocket(execUrl)
    wsRef.current = ws

    ws.onopen = () => {
      console.log('Pod terminal: connected')
      // Send initial resize
      const { rows, cols } = term
      ws.send(JSON.stringify({ action: 'resize', rows, cols }))
    }

    ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data)
        if (message.action === 'stdout' && message.data) {
          term.write(message.data)
        } else if (message.action === 'stderr' && message.data) {
          term.write(message.data)
        }
      } catch (err) {
        console.error('Pod terminal: failed to parse message', err)
      }
    }

    ws.onclose = () => {
      console.log('Pod terminal: disconnected')
      term.write('\r\n\x1b[33mConnection closed\x1b[0m\r\n')
    }

    ws.onerror = (error) => {
      console.error('Pod terminal error:', error)
      term.write('\r\n\x1b[31mConnection error\x1b[0m\r\n')
    }

    // Handle terminal input
    term.onData(handleData)

    // Handle resize
    term.onResize(handleResize)

    // Observe terminal container resize
    const resizeObserver = new ResizeObserver(() => {
      fitAddon.fit()
      const { rows, cols } = term
      handleResize({ rows, cols })
    })
    resizeObserver.observe(terminalRef.current)
    resizeObserverRef.current = resizeObserver

    return () => {
      resizeObserver.disconnect()
      ws.close()
      term.dispose()
      onClose?.()
    }
  }, [podName, containerName, namespace, token, handleData, handleResize, onClose])

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <span className={styles.title}>Terminal: {podName}</span>
        <button className={styles.closeButton} onClick={onClose}>
          Close
        </button>
      </div>
      <div ref={terminalRef} className={styles.terminal} />
    </div>
  )
}

export default PodTerminal