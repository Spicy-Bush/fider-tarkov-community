import { useEffect, useRef } from "react"
import { reportsEventSource } from "@fider/services"

export function useReportsEvents(refresh: () => Promise<void>): void {
  const currentRefresh = useRef(refresh)
  currentRefresh.current = refresh

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | undefined
    let running = false
    let changed = false
    let disposed = false

    const reload = async () => {
      if (running || disposed) {
        return
      }

      running = true
      changed = false

      try {
        await currentRefresh.current()
      } finally {
        running = false

        if (changed && !disposed) {
          timer = setTimeout(reload, 100)
        }
      }
    }

    const invalidate = () => {
      changed = true

      if (running) {
        return
      }

      clearTimeout(timer)
      timer = setTimeout(reload, 100)
    }

    reportsEventSource.connect()
    const unsubscribe = reportsEventSource.on("reports.changed", invalidate)
    const reconnect = reportsEventSource.on("connection.open", invalidate)

    return () => {
      disposed = true
      clearTimeout(timer)
      unsubscribe()
      reconnect()
      reportsEventSource.disconnect()
    }
  }, [])
}
