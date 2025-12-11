export interface ConnectionEvent {
  type: "open" | "error" | "closed" | "failed"
}

export interface ViewerInfo {
  userId: number
  userName: string
}

export interface QueuePostNewEvent {
  postId: number
  postNumber?: number
  untaggedByUserId?: number
}

export interface QueuePostTaggedEvent {
  postId: number
  postNumber: number
  tags: string[]
  taggedByUserId: number
}

export interface QueueViewerJoinedEvent {
  postId: number
  userId: number
  userName: string
}

export interface QueueViewerLeftEvent {
  postId: number
  userId: number
}

export interface ReportNewEvent {
  reportId: number
  reportedType: string
  reportedId: number
  reason: string
}

export interface ReportAssignedEvent {
  reportId: number
  assignedTo: {
    userId: number
    userName: string
  }
}

export interface ReportUnassignedEvent {
  reportId: number
}

export interface ReportResolvedEvent {
  reportId: number
  status: string
}

export interface ReportViewerJoinedEvent {
  reportId: number
  userId: number
  userName: string
}

export interface ReportViewerLeftEvent {
  reportId: number
  userId: number
}

export type SSEEventMap = {
  "connection.open": Record<string, never>
  "connection.error": Record<string, never>
  "connection.closed": Record<string, never>
  "connection.failed": Record<string, never>

  "queue.post_new": QueuePostNewEvent
  "queue.post_tagged": QueuePostTaggedEvent
  "queue.viewer_joined": QueueViewerJoinedEvent
  "queue.viewer_left": QueueViewerLeftEvent

  "report.new": ReportNewEvent
  "report.assigned": ReportAssignedEvent
  "report.unassigned": ReportUnassignedEvent
  "report.resolved": ReportResolvedEvent
  "report.viewer_joined": ReportViewerJoinedEvent
  "report.viewer_left": ReportViewerLeftEvent
}

export type SSEEventType = keyof SSEEventMap

export type SSEEventHandler<T extends SSEEventType> = (
  type: T,
  payload: SSEEventMap[T]
) => void

