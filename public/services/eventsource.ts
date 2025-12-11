import { http } from "./http"
import { TIME, RETRY } from "./constants"

type MessageHandler = (type: string, payload: unknown) => void

interface EventSourceMessage {
  type: string
  payload: unknown
}

interface HeartbeatConfig {
  heartbeatEndpoint: (itemId: number) => string
  stopViewingEndpoint: string
}

interface EventSourceConfig {
  endpoint: string
  heartbeatConfig?: HeartbeatConfig
}

interface EventSourceInstance {
  connect: () => void
  disconnect: () => void
  on: (type: string, handler: MessageHandler) => () => void
  isConnected: () => boolean
  viewItem: (itemId: number) => void
  stopViewing: () => void
}

export const createEventSource = (config: EventSourceConfig): EventSourceInstance => {
  const { endpoint, heartbeatConfig } = config

  let es: EventSource | null = null
  const handlers = new Map<string, Set<MessageHandler>>()
  let reconnectAttempts = 0
  let reconnectDelay = TIME.RECONNECT_DELAY_MS
  let isIntentionallyClosed = false
  let heartbeatInterval: ReturnType<typeof setInterval> | null = null
  let viewingItemId: number | null = null
  let visibilityChangeHandler: (() => void) | null = null
  let connectionCount = 0

  const emit = (type: string, payload: unknown): void => {
    const typeHandlers = handlers.get(type)
    if (typeHandlers) {
      typeHandlers.forEach((handler) => handler(type, payload))
    }

    const wildcardHandlers = handlers.get("*")
    if (wildcardHandlers) {
      wildcardHandlers.forEach((handler) => handler(type, payload))
    }
  }

  const sendHeartbeat = (): void => {
    if (!isPageVisible() || !viewingItemId || !heartbeatConfig) return
    http.post(heartbeatConfig.heartbeatEndpoint(viewingItemId)).catch(() => {})
  }

  const isPageVisible = (): boolean => {
    return typeof document !== "undefined" && !document.hidden
  }

  const stopHeartbeat = (): void => {
    if (heartbeatInterval) {
      clearInterval(heartbeatInterval)
      heartbeatInterval = null
    }
  }

  const removeVisibilityListener = (): void => {
    if (visibilityChangeHandler && typeof document !== "undefined") {
      document.removeEventListener("visibilitychange", visibilityChangeHandler)
      visibilityChangeHandler = null
    }
  }

  const setupVisibilityListener = (): void => {
    removeVisibilityListener()
    if (typeof document === "undefined") return

    visibilityChangeHandler = () => {
      if (isPageVisible()) {
        if (!heartbeatInterval && viewingItemId) {
          heartbeatInterval = setInterval(() => {
            if (isPageVisible()) sendHeartbeat()
          }, TIME.HEARTBEAT_INTERVAL_MS)
        }
        if (viewingItemId) sendHeartbeat()
      } else {
        stopHeartbeat()
      }
    }

    document.addEventListener("visibilitychange", visibilityChangeHandler)
  }

  const startHeartbeat = (): void => {
    stopHeartbeat()
    setupVisibilityListener()

    if (isPageVisible()) {
      heartbeatInterval = setInterval(() => {
        if (isPageVisible()) sendHeartbeat()
      }, TIME.HEARTBEAT_INTERVAL_MS)
    }
  }

  const onReconnected = (): void => {
    if (viewingItemId && heartbeatConfig) {
      sendHeartbeat()
    }
  }

  const scheduleReconnect = (): void => {
    if (reconnectAttempts >= RETRY.MAX_ATTEMPTS || isIntentionallyClosed) {
      emit("connection.failed", {})
      return
    }

    reconnectAttempts++
    const delay = Math.min(reconnectDelay * Math.pow(2, reconnectAttempts - 1), TIME.MAX_RECONNECT_DELAY_MS)

    setTimeout(() => {
      if (!isIntentionallyClosed && es === null) {
        connect()
      }
    }, delay)
  }

  const connect = (): void => {
    connectionCount++

    if (es !== null) return

    isIntentionallyClosed = false

    try {
      es = new EventSource(endpoint)

      es.onopen = () => {
        reconnectAttempts = 0
        reconnectDelay = TIME.RECONNECT_DELAY_MS
        onReconnected()
        emit("connection.open", {})
      }

      es.onmessage = (event) => {
        try {
          const message: EventSourceMessage = JSON.parse(event.data)
          emit(message.type, message.payload)
        } catch {}
      }

      es.onerror = () => {
        emit("connection.error", {})

        if (es?.readyState === EventSource.CLOSED) {
          es = null
          if (!isIntentionallyClosed) {
            emit("connection.closed", {})
            scheduleReconnect()
          }
        }
      }
    } catch {
      es = null
      scheduleReconnect()
    }
  }

  const disconnect = (): void => {
    connectionCount = Math.max(0, connectionCount - 1)

    if (connectionCount > 0) return

    isIntentionallyClosed = true
    stopHeartbeat()
    removeVisibilityListener()
    viewingItemId = null
    if (es) {
      es.close()
      es = null
    }
  }

  const on = (type: string, handler: MessageHandler): (() => void) => {
    if (!handlers.has(type)) {
      handlers.set(type, new Set())
    }
    handlers.get(type)!.add(handler)

    return () => {
      handlers.get(type)?.delete(handler)
    }
  }

  const isConnected = (): boolean => {
    return es?.readyState === EventSource.OPEN
  }

  const viewItem = (itemId: number): void => {
    if (!heartbeatConfig) return

    const isNewItem = viewingItemId !== itemId
    viewingItemId = itemId

    sendHeartbeat()

    if (isNewItem) {
      startHeartbeat()
    }
  }

  const stopViewing = (): void => {
    if (!heartbeatConfig) return

    stopHeartbeat()
    removeVisibilityListener()
    if (viewingItemId) {
      viewingItemId = null
      http.delete(heartbeatConfig.stopViewingEndpoint).catch(() => {})
    }
  }

  return {
    connect,
    disconnect,
    on,
    isConnected,
    viewItem,
    stopViewing,
  }
}

interface ReportsEventSource extends EventSourceInstance {
  viewReport: (reportId: number) => void
  stopViewingReport: () => void
}

interface QueueEventSource extends EventSourceInstance {
  viewPost: (postId: number) => void
  stopViewingPost: () => void
}

const createReportsEventSource = (): ReportsEventSource => {
  const instance = createEventSource({
    endpoint: "/api/mod/report-events",
    heartbeatConfig: {
      heartbeatEndpoint: (reportId) => `/api/v1/reports/${reportId}/heartbeat`,
      stopViewingEndpoint: "/api/mod/viewing",
    },
  })

  return {
    ...instance,
    viewReport: (reportId: number) => instance.viewItem(reportId),
    stopViewingReport: () => instance.stopViewing(),
  }
}

const createQueueEventSource = (): QueueEventSource => {
  const instance = createEventSource({
    endpoint: "/api/mod/queue-events",
    heartbeatConfig: {
      heartbeatEndpoint: (postId) => `/api/v1/queue/${postId}/heartbeat`,
      stopViewingEndpoint: "/api/mod/queue-viewing",
    },
  })

  return {
    ...instance,
    viewPost: (postId: number) => instance.viewItem(postId),
    stopViewingPost: () => instance.stopViewing(),
  }
}

export const reportsEventSource = createReportsEventSource()
export const queueEventSource = createQueueEventSource()
