import { useEffect, useRef } from "react"

type EventHandler<T = unknown> = (type: string, payload: T) => void

interface EventSource {
  on: (type: string, handler: EventHandler<unknown>) => () => void
  connect: () => void
  disconnect: () => void
}

type EventHandlerMap = Record<string, EventHandler<unknown>>

export const useRealtimeEvents = <T extends EventHandlerMap>(
  eventSource: EventSource,
  handlers: T
): void => {
  const handlersRef = useRef(handlers)

  useEffect(() => {
    handlersRef.current = handlers
  }, [handlers])

  useEffect(() => {
    eventSource.connect()

    const unsubscribers: Array<() => void> = []

    for (const [eventType, handler] of Object.entries(handlersRef.current)) {
      const unsubscribe = eventSource.on(eventType, (type, payload) => {
        const currentHandler = handlersRef.current[eventType]
        if (currentHandler) {
          currentHandler(type, payload)
        }
      })
      unsubscribers.push(unsubscribe)
    }

    return () => {
      unsubscribers.forEach((unsub) => unsub())
      eventSource.disconnect()
    }
  }, [eventSource])
}

