import { useEffect, useRef, useCallback } from 'react'
import type { WSMessage } from '../api/types'

interface WebSocketHookReturn {
  send: (data: unknown) => void
  disconnect: () => void
}

function buildWSURL(path: string): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}${path}`
}

export function useWebSocket(
  path: string,
  onMessage: (msg: WSMessage) => void,
): WebSocketHookReturn {
  const wsRef = useRef<WebSocket | null>(null)
  const onMessageRef = useRef(onMessage)
  // True between a manual disconnect() and the next connect(); the close
  // handler must not schedule a reconnect for an intentional close.
  const manualCloseRef = useRef(false)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    onMessageRef.current = onMessage
  }, [onMessage])

  const clearReconnectTimer = useCallback(() => {
    if (reconnectTimerRef.current !== null) {
      clearTimeout(reconnectTimerRef.current)
      reconnectTimerRef.current = null
    }
  }, [])

  const connect = useCallback(() => {
    clearReconnectTimer()
    manualCloseRef.current = false
    if (wsRef.current?.readyState === WebSocket.OPEN) return

    const url = buildWSURL(path)
    const ws = new WebSocket(url)

    ws.onopen = () => {
      console.log(`[WS] connected to ${path}`)
    }

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data) as WSMessage
        onMessageRef.current(msg)
      } catch (err) {
        console.error('[WS] parse error:', err)
      }
    }

    ws.onclose = () => {
      if (manualCloseRef.current) return
      console.log(`[WS] disconnected from ${path}, reconnecting...`)
      reconnectTimerRef.current = setTimeout(connect, 3000)
    }

    ws.onerror = (err) => {
      console.error('[WS] error:', err)
    }

    wsRef.current = ws
  }, [path, clearReconnectTimer])

  const disconnect = useCallback(() => {
    manualCloseRef.current = true
    clearReconnectTimer()
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }
  }, [clearReconnectTimer])

  const send = useCallback((data: unknown) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(data))
    }
  }, [])

  useEffect(() => {
    connect()
    return () => disconnect()
  }, [connect, disconnect])

  return { send, disconnect }
}
