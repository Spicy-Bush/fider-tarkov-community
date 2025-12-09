import { http } from "./http"

type MessageHandler = (type: string, payload: unknown) => void

interface EventSourceMessage {
  type: string
  payload: unknown
}

class ModEventSourceService {
  private es: EventSource | null = null
  private handlers: Map<string, Set<MessageHandler>> = new Map()
  private reconnectAttempts = 0
  private maxReconnectAttempts = 10
  private reconnectDelay = 1000
  private isIntentionallyClosed = false
  private heartbeatInterval: ReturnType<typeof setInterval> | null = null
  private viewingReportId: number | null = null

  connect(): void {
    if (this.es !== null) {
      return
    }

    this.isIntentionallyClosed = false

    try {
      this.es = new EventSource("/api/mod/events")

      this.es.onopen = () => {
        this.reconnectAttempts = 0
        this.reconnectDelay = 1000
        // re announce presence if we were viewing a report before disconnect
        if (this.viewingReportId) {
          this.sendHeartbeat()
        }
        this.emit("connection.open", {})
      }

      this.es.onmessage = (event) => {
        try {
          const message: EventSourceMessage = JSON.parse(event.data)
          this.emit(message.type, message.payload)
        } catch {}
      }

      this.es.onerror = () => {
        this.emit("connection.error", {})

        if (this.es?.readyState === EventSource.CLOSED) {
          this.es = null
          if (!this.isIntentionallyClosed) {
            this.emit("connection.closed", {})
            this.scheduleReconnect()
          }
        }
      }
    } catch {
      this.es = null
      this.scheduleReconnect()
    }
  }

  private scheduleReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts || this.isIntentionallyClosed) {
      this.emit("connection.failed", {})
      return
    }

    this.reconnectAttempts++
    const delay = Math.min(this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1), 30000)

    setTimeout(() => {
      if (!this.isIntentionallyClosed && this.es === null) {
        this.connect()
      }
    }, delay)
  }

  disconnect(): void {
    this.isIntentionallyClosed = true
    this.stopHeartbeat()
    this.viewingReportId = null
    if (this.es) {
      this.es.close()
      this.es = null
    }
  }

  on(type: string, handler: MessageHandler): () => void {
    if (!this.handlers.has(type)) {
      this.handlers.set(type, new Set())
    }
    this.handlers.get(type)!.add(handler)

    return () => {
      this.handlers.get(type)?.delete(handler)
    }
  }

  private emit(type: string, payload: unknown): void {
    const handlers = this.handlers.get(type)
    if (handlers) {
      handlers.forEach((handler) => handler(type, payload))
    }

    const wildcardHandlers = this.handlers.get("*")
    if (wildcardHandlers) {
      wildcardHandlers.forEach((handler) => handler(type, payload))
    }
  }

  viewReport(reportId: number): void {
    const isNewReport = this.viewingReportId !== reportId
    this.viewingReportId = reportId

    this.sendHeartbeat()

    if (isNewReport) {
      this.startHeartbeat()
    }
  }

  stopViewingReport(): void {
    this.stopHeartbeat()
    if (this.viewingReportId) {
      this.viewingReportId = null
      http.delete("/api/mod/viewing")
    }
  }

  private startHeartbeat(): void {
    this.stopHeartbeat()
    this.heartbeatInterval = setInterval(() => {
      this.sendHeartbeat()
    }, 30000)
  }

  private stopHeartbeat(): void {
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval)
      this.heartbeatInterval = null
    }
  }

  private sendHeartbeat(): void {
    if (this.viewingReportId) {
      http.post(`/api/v1/reports/${this.viewingReportId}/heartbeat`)
    }
  }

  isConnected(): boolean {
    return this.es?.readyState === EventSource.OPEN
  }
}

export const modEventSource = new ModEventSourceService()
