import { useEffect, useRef, useCallback } from 'react'
import { create } from 'zustand'

interface WebSocketState {
    isConnected: boolean
    lastMessage: unknown | null
    setConnected: (connected: boolean) => void
    setLastMessage: (message: unknown) => void
}

export const useWebSocketStore = create<WebSocketState>((set) => ({
    isConnected: false,
    lastMessage: null,
    setConnected: (connected) => set({ isConnected: connected }),
    setLastMessage: (message) => set({ lastMessage: message }),
}))

interface UseWebSocketOptions {
    url: string
    onMessage?: (data: unknown) => void
    onOpen?: () => void
    onClose?: () => void
    reconnectInterval?: number
    maxReconnectAttempts?: number
}

export function useWebSocket(options: UseWebSocketOptions) {
    const {
        url,
        onMessage,
        onOpen,
        onClose,
        reconnectInterval = 3000,
        maxReconnectAttempts = 5,
    } = options

    const ws = useRef<WebSocket | null>(null)
    const reconnectCount = useRef(0)
    const reconnectTimer = useRef<NodeJS.Timeout>()

    const { setConnected, setLastMessage } = useWebSocketStore()

    const connect = useCallback(() => {
        if (ws.current?.readyState === WebSocket.OPEN) return

        try {
            ws.current = new WebSocket(url)

            ws.current.onopen = () => {
                console.log('[WS] Connected')
                setConnected(true)
                reconnectCount.current = 0
                onOpen?.()
            }

            ws.current.onmessage = (event) => {
                try {
                    const data = JSON.parse(event.data)
                    setLastMessage(data)
                    onMessage?.(data)
                } catch {
                    console.error('[WS] Failed to parse message')
                }
            }

            ws.current.onclose = () => {
                console.log('[WS] Disconnected')
                setConnected(false)
                onClose?.()

                // Attempt reconnection
                if (reconnectCount.current < maxReconnectAttempts) {
                    reconnectCount.current++
                    console.log(`[WS] Reconnecting... (${reconnectCount.current}/${maxReconnectAttempts})`)
                    reconnectTimer.current = setTimeout(connect, reconnectInterval)
                }
            }

            ws.current.onerror = (error) => {
                console.error('[WS] Error:', error)
            }
        } catch (error) {
            console.error('[WS] Failed to connect:', error)
        }
    }, [url, onMessage, onOpen, onClose, reconnectInterval, maxReconnectAttempts, setConnected, setLastMessage])

    const disconnect = useCallback(() => {
        if (reconnectTimer.current) {
            clearTimeout(reconnectTimer.current)
        }
        if (ws.current) {
            ws.current.close()
            ws.current = null
        }
    }, [])

    const send = useCallback((data: unknown) => {
        if (ws.current?.readyState === WebSocket.OPEN) {
            ws.current.send(JSON.stringify(data))
        }
    }, [])

    useEffect(() => {
        connect()
        return () => disconnect()
    }, [connect, disconnect])

    return { connect, disconnect, send }
}
