import { useEffect, useRef, useCallback } from 'react'

export interface WsEvent {
  type: string
  payload: unknown
}

export type WsHandler = (event: WsEvent) => void

const WS_URL = `ws://${window.location.host}/ws`
const RECONNECT_DELAY_MS = 3000

export function useWebSocket(onEvent: WsHandler) {
  const onEventRef = useRef(onEvent)
  onEventRef.current = onEvent

  const connect = useCallback(() => {
    const ws = new WebSocket(WS_URL)

    ws.onmessage = (e) => {
      try {
        const event = JSON.parse(e.data) as WsEvent
        onEventRef.current(event)
      } catch {
        // ignore malformed frames
      }
    }

    ws.onclose = () => {
      setTimeout(connect, RECONNECT_DELAY_MS)
    }

    return ws
  }, [])

  useEffect(() => {
    const ws = connect()
    return () => ws.close()
  }, [connect])
}
