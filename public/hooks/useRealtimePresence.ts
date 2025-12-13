import { useState, useEffect, useCallback, useRef } from "react"
import { reportsEventSource, queueEventSource } from "@fider/services"
import { ViewerInfo } from "@fider/models"

export type PresenceEventPrefix = "queue" | "report"

interface EventSourceLike {
  connect: () => void
  disconnect: () => void
  isConnected: () => boolean
  on: (type: string, handler: (type: string, payload: unknown) => void) => () => void
  viewItem?: (itemId: number) => void
  stopViewing?: () => void
}

export interface InitialViewers {
  postId?: number
  reportId?: number
  viewers: ViewerInfo[]
}

export interface UseRealtimePresenceOptions {
  eventPrefix: PresenceEventPrefix
  itemIdField?: string
  initialViewers?: InitialViewers[]
  eventSource?: EventSourceLike
}

export interface UseRealtimePresenceResult {
  viewers: Map<number, ViewerInfo[]>
  viewItem: (itemId: number) => void
  stopViewing: () => void
  isConnected: boolean
  setInitialViewers: (viewers: InitialViewers[]) => void
}

export function useRealtimePresence(
  options: UseRealtimePresenceOptions
): UseRealtimePresenceResult {
  const { eventPrefix, itemIdField = "postId", initialViewers, eventSource: customEventSource } = options
  
  const buildInitialMap = useCallback(() => {
    const map = new Map<number, ViewerInfo[]>()
    if (initialViewers) {
      for (const item of initialViewers) {
        const id = item.postId ?? item.reportId ?? 0
        if (id && item.viewers.length > 0) {
          map.set(id, item.viewers)
        }
      }
    }
    return map
  }, [initialViewers])

  const [viewers, setViewers] = useState<Map<number, ViewerInfo[]>>(buildInitialMap)
  const [isConnected, setIsConnected] = useState(false)
  const connectedRef = useRef(false)

  const defaultEventSource = eventPrefix === "queue" ? queueEventSource : reportsEventSource
  const eventSource = customEventSource ?? defaultEventSource

  const viewItem = useCallback(
    (itemId: number) => {
      if (eventSource.viewItem) {
        eventSource.viewItem(itemId)
      } else if (eventPrefix === "queue") {
        queueEventSource.viewPost(itemId)
      } else {
        reportsEventSource.viewReport(itemId)
      }
    },
    [eventPrefix, eventSource]
  )

  const stopViewing = useCallback(() => {
    if (eventSource.stopViewing) {
      eventSource.stopViewing()
    } else if (eventPrefix === "queue") {
      queueEventSource.stopViewingPost()
    } else {
      reportsEventSource.stopViewingReport()
    }
  }, [eventPrefix, eventSource])

  const setInitialViewers = useCallback((newViewers: InitialViewers[]) => {
    const map = new Map<number, ViewerInfo[]>()
    for (const item of newViewers) {
      const id = item.postId ?? item.reportId ?? 0
      if (id && item.viewers.length > 0) {
        map.set(id, item.viewers)
      }
    }
    setViewers(map)
  }, [])

  useEffect(() => {
    if (!connectedRef.current) {
      eventSource.connect()
      connectedRef.current = true
    }

    const handleOpen = () => {
      setIsConnected(true)
    }

    const handleClosed = () => {
      setIsConnected(false)
    }

    const handleViewerJoined = (_: string, payload: unknown) => {
      const data = payload as Record<string, unknown>
      const id = (data[itemIdField] ?? data.reportId ?? data.postId) as number
      const userId = data.userId as number
      const userName = data.userName as string

      setViewers((prev) => {
        const newMap = new Map(prev)
        const current = newMap.get(id) || []
        if (!current.find((v) => v.userId === userId)) {
          newMap.set(id, [...current, { userId, userName }])
        }
        return newMap
      })
    }

    const handleViewerLeft = (_: string, payload: unknown) => {
      const data = payload as Record<string, unknown>
      const id = (data[itemIdField] ?? data.reportId ?? data.postId) as number
      const userId = data.userId as number

      setViewers((prev) => {
        const newMap = new Map(prev)
        const current = newMap.get(id) || []
        newMap.set(id, current.filter((v) => v.userId !== userId))
        return newMap
      })
    }

    const unsubOpen = eventSource.on("connection.open", handleOpen)
    const unsubClosed = eventSource.on("connection.closed", handleClosed)
    const unsubViewerJoined = eventSource.on(`${eventPrefix}.viewer_joined`, handleViewerJoined)
    const unsubViewerLeft = eventSource.on(`${eventPrefix}.viewer_left`, handleViewerLeft)

    if (eventSource.isConnected()) {
      setIsConnected(true)
    }

    return () => {
      unsubOpen()
      unsubClosed()
      unsubViewerJoined()
      unsubViewerLeft()
    }
  }, [eventPrefix, itemIdField, eventSource])

  useEffect(() => {
    return () => {
      stopViewing()
      eventSource.disconnect()
      connectedRef.current = false
    }
  }, [eventSource, stopViewing])

  return {
    viewers,
    viewItem,
    stopViewing,
    isConnected,
    setInitialViewers,
  }
}
