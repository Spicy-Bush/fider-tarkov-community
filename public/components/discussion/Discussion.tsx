import React, { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react"
import { Button } from "@fider/components/common/Button"
import { SignInModal } from "@fider/components/auth/SignInModal"
import { DiscussionComment, DiscussionOwner, DiscussionPage, DiscussionPermissions, DiscussionSort, ReactionCount } from "@fider/models"
import { isNegativelyRated, loadCommentContext, loadCommentRecords, loadComments } from "@fider/services/discussion"
import { savedReadingPosition, useReadingPosition } from "@fider/services/readingPosition"
import { RequestError } from "@fider/services/http"
import { CommentComposer } from "./CommentComposer"
import { CommentChange, CommentInteraction, DiscussionCommentCard } from "./DiscussionCommentCard"
import { DiscussionRow, DiscussionViewport } from "./DiscussionViewport"

interface Branch {
  ids: number[]
  page: { next?: string } | null
}

type BranchRead =
  | { status: "pending"; controller: AbortController }
  | { status: "failed"; message: string }

interface DiscussionState {
  comments: Record<number, DiscussionComment | PendingComment>
  branches: Record<number, Branch>
  permissions: DiscussionPermissions
  permissionRead: number
  recordVersions: Record<number, number>
  reads: Record<number, BranchRead>
}

interface PendingComment {
  id: number
  parentId: number | null
  hasReplies: boolean
  collapsed: boolean
  pending: "unloaded" | "unavailable"
}

interface DiscussionPosition {
  sort: DiscussionSort
  comments: PendingComment[]
  branches: Record<number, { ids: number[]; next?: string }>
  collapsed: Record<number, boolean>
  expanded: Record<number, boolean>
  measurements: [string, number][]
  viewport?: { top: number; bottom: number }
  anchor?: { id: number; top: number }
}

interface DiscussionProps {
  owner: DiscussionOwner
  ownerPermissions: DiscussionPermissions
  onCommentAdded?: () => void
  headerActions?: React.ReactNode
}

interface ThreadContext {
  commentId: number
  rootId?: number
  nextAncestorId?: number
}

function mergeRecords(state: DiscussionState, comments: DiscussionComment[], version: number): DiscussionState {
  const next = { ...state, comments: { ...state.comments }, recordVersions: { ...state.recordVersions } }

  for (const comment of comments) {
    const current = state.comments[comment.id]
    const currentVersion = state.recordVersions[comment.id] || 0
    let accepted = currentVersion > version ? current : comment

    if (current && !("pending" in current)) {
      if (current.state === "deleted") {
        accepted = current
      } else if (comment.state === "deleted") {
        accepted = comment
      } else if (current.state === "visible" && comment.state === "visible" && (current.editedAt || comment.editedAt)) {
        const edited = hasNewerEdit(comment, current) ? comment : current
        accepted = {
          ...accepted,
          content: edited.content,
          attachments: edited.attachments,
          editedAt: edited.editedAt,
          editedBy: edited.editedBy,
        }
      }
    }

    next.comments[comment.id] = accepted
    next.recordVersions[comment.id] = Math.max(currentVersion, version)
  }

  return next
}

function acceptPermissions(state: DiscussionState, page: DiscussionPage, read: number): DiscussionState {
  return read < state.permissionRead ? state : { ...state, permissionRead: read, permissions: page.permissions }
}

function connectComments(state: DiscussionState, comments: DiscussionComment[]): DiscussionState {
  const additions = new Map<number, number[]>()

  for (const comment of comments) {
    const parent = comment.parentId || 0
    const ids = additions.get(parent)

    if (ids) {
      ids.push(comment.id)
    } else {
      additions.set(parent, [comment.id])
    }
  }

  const branches = { ...state.branches }

  for (const [parent, incoming] of additions) {
    const branch = branches[parent] || { ids: [], page: null }
    branches[parent] = { ...branch, ids: [...new Set([...branch.ids, ...incoming])] }
  }

  return { ...state, branches }
}

function hasNewerEdit(incoming: DiscussionComment, current: DiscussionComment): boolean {
  if (!current.editedAt) {
    return true
  }

  if (!incoming.editedAt) {
    return false
  }

  const difference = Date.parse(incoming.editedAt) - Date.parse(current.editedAt)

  if (difference !== 0) {
    return difference > 0
  }

  // PostgreSQL edit ordering retains precision beyond JavaScript milliseconds.
  const fraction = (value: string) => (/\.(\d+)/.exec(value)?.[1] || "").padEnd(9, "0")
  return fraction(incoming.editedAt) > fraction(current.editedAt)
}

export function discussionRows(
  state: Pick<DiscussionState, "comments" | "branches">,
  collapsed: Record<number, boolean>,
  focusedRoot?: number,
  expanded: Record<number, boolean> = {}
): DiscussionRow[] {
  const rows: DiscussionRow[] = []
  type PendingRow =
    | { kind: "comment"; id: number; depth: number; limit: number }
    | { kind: "branch"; parentId: number; depth: number; limit: number }
    | { kind: "load"; parentId: number; depth: number; levels: number }
    | { kind: "end"; comment: Extract<DiscussionRow, { kind: "comment" }> }

  const pending: PendingRow[] = focusedRoot
    ? [{ kind: "comment", id: focusedRoot, depth: 0, limit: 4 }]
    : [{ kind: "branch", parentId: 0, depth: 0, limit: 4 }]

  while (pending.length > 0) {
    const row = pending.pop()!

    if (row.kind === "end") {
      row.comment.end = rows.length
      continue
    }

    if (row.kind === "branch") {
      const branch = state.branches[row.parentId]

      if (!branch?.page || branch.page.next) {
        pending.push({ kind: "load", parentId: row.parentId, depth: row.depth, levels: row.limit - row.depth + 1 })
      }

      if (branch) {
        for (let index = branch.ids.length - 1; index >= 0; index--) {
          pending.push({ kind: "comment", id: branch.ids[index], depth: row.depth, limit: row.limit })
        }
      }

      continue
    }

    if (row.kind === "load") {
      rows.push(row)
      continue
    }

    const comment = state.comments[row.id]
    const closed = collapsed[row.id] ?? ("pending" in comment ? comment.collapsed : isNegativelyRated(comment))
    const displayed: Extract<DiscussionRow, { kind: "comment" }> = {
      kind: "comment",
      id: row.id,
      depth: row.depth,
      end: rows.length + 1,
      collapsed: closed,
    }
    rows.push(displayed)

    if (!closed && (comment.hasReplies || state.branches[row.id]?.ids.length)) {
      pending.push({ kind: "end", comment: displayed })
      const limit = expanded[row.id] ? row.depth + 10 : row.limit

      if (row.depth >= limit) {
        rows.push({ kind: "continue", parentId: row.id, depth: row.depth })
      } else {
        pending.push({ kind: "branch", parentId: row.id, depth: row.depth + 1, limit })
      }
    }
  }

  return rows
}

function LoadComments(props: { read?: BranchRead; load: () => Promise<void> }) {
  const ref = useRef<HTMLDivElement>(null)
  const loading = props.read?.status === "pending"
  const error = props.read?.status === "failed" ? props.read.message : undefined

  useEffect(() => {
    if (!ref.current || error || loading) {
      return
    }

    const observer = new IntersectionObserver((entries) => {
      if (entries.some((entry) => entry.isIntersecting)) {
        void props.load()
      }
    }, { rootMargin: "300px" })

    observer.observe(ref.current)
    return () => observer.disconnect()
  }, [props.load, error, loading])

  return (
    <div ref={ref} className="py-3">
      {error && <p role="alert" className="text-danger">{error}</p>}
      <Button size="small" loading={loading} onClick={props.load}>
        {error ? "Retry loading comments" : "Load more comments"}
      </Button>
    </div>
  )
}

function linkedComment(): number | undefined {
  const match = /^#comment-(\d+)$/.exec(window.location.hash)
  return match ? Number(match[1]) : undefined
}

export function Discussion(props: DiscussionProps) {
  return <DiscussionThreads key={`${props.owner.kind}:${props.owner.id}`} {...props} />
}

function DiscussionThreads({ owner, ownerPermissions, onCommentAdded, headerActions }: DiscussionProps) {
  const container = useRef<HTMLElement>(null)
  const positionKey = `discussion:${owner.kind}:${owner.id}`
  const [sort, setSort] = useState<DiscussionSort>(() => savedReadingPosition<DiscussionPosition>(positionKey)?.sort ?? "liked")
  const reading = useReadingPosition<DiscussionPosition>(positionKey, (): DiscussionPosition => {
    const anchor = [...(container.current?.querySelectorAll<HTMLElement>(".discussion-row article[id]") ?? [])]
      .find((element) => element.getBoundingClientRect().bottom > 0)
    const viewport = container.current?.querySelector(".discussion-threads")?.getBoundingClientRect()
    const branches = Object.fromEntries(
      Object.entries(state.branches)
        .filter(([, branch]) => branch.ids.length > 0)
        .map(([id, branch]) => [id, { ids: branch.ids, next: branch.page?.next }])
    )

    return {
      sort,
      comments: Object.values(state.comments).map((comment) => ({
        id: comment.id,
        parentId: comment.parentId,
        hasReplies: comment.hasReplies,
        collapsed: "pending" in comment ? comment.collapsed : isNegativelyRated(comment),
        pending: "unloaded",
      })),
      branches,
      collapsed,
      expanded,
      measurements: [...measurements.current],
      viewport: viewport ? { top: Math.max(0, -viewport.top), bottom: window.innerHeight - viewport.top } : undefined,
      anchor: anchor ? { id: Number(anchor.id.slice("comment-".length)), top: anchor.getBoundingClientRect().top } : undefined,
    }
  })
  const [state, renderState] = useState<DiscussionState>(() => ({
    comments: Object.fromEntries(reading.saved?.comments.map((comment) => [comment.id, comment]) ?? []),
    branches: Object.fromEntries(
      Object.entries(reading.saved?.branches ?? {}).map(([id, branch]) => [
        id,
        { ids: branch.ids, page: { next: branch.next } },
      ])
    ),
    reads: {},
    recordVersions: {},
    permissionRead: 0,
    permissions: ownerPermissions,
  }))
  const stateRef = useRef(state)
  const setState = useCallback((change: (previous: DiscussionState) => DiscussionState) => {
    stateRef.current = change(stateRef.current)
    renderState(stateRef.current)
  }, [])
  const nextVersion = useRef(0)

  useLayoutEffect(() => {
    const permissionRead = ++nextVersion.current
    setState((previous) => ({ ...previous, permissions: ownerPermissions, permissionRead }))
  }, [ownerPermissions, setState])

  const [collapsed, setCollapsed] = useState<Record<number, boolean>>(reading.saved?.collapsed ?? {})
  const [expanded, setExpanded] = useState<Record<number, boolean>>(reading.saved?.expanded ?? {})
  const measurements = useRef(new Map(reading.saved?.measurements))
  const initialViewport = useRef(reading.saved?.viewport)
  const [readingTarget, setReadingTarget] = useState(reading.saved?.anchor?.id)
  const [recordsError, setRecordsError] = useState<string>()
  const [visibleRecords, setVisibleRecords] = useState<number[]>([])
  const [context, setContext] = useState<ThreadContext>()
  const [target, setTarget] = useState(linkedComment)
  const [contextError, setContextError] = useState<string>()
  const [signIn, setSignIn] = useState(false)
  const [interactions, setInteractions] = useState<Record<number, CommentInteraction>>({})
  // Scrolling existing comments into view must not replay entry animations.
  const [entering, setEntering] = useState<ReadonlySet<number>>(new Set())
  const activeActions = useRef(new Set<number>())
  const reportedComments = useRef(new Set<number>())
  const recordRequest = useRef<AbortController>()
  const contextRequest = useRef<AbortController>()
  const unread = useMemo(() => visibleRecords.filter((id) => {
    const comment = state.comments[id]
    return comment && "pending" in comment && comment.pending === "unloaded"
  }).slice(0, 50), [visibleRecords, state.comments])

  useEffect(() => {
    initialViewport.current = undefined
  }, [])

  const abortReads = useCallback(() => {
    for (const read of Object.values(stateRef.current.reads)) {
      if (read.status === "pending") {
        read.controller.abort()
      }
    }
    recordRequest.current?.abort()
    recordRequest.current = undefined
    contextRequest.current?.abort()
  }, [])

  useEffect(() => abortReads, [abortReads])

  const loadBranch = useCallback(async (parentId: number, levels: number) => {
    if (stateRef.current.reads[parentId]?.status === "pending") {
      return
    }

    const controller = new AbortController()
    const read = ++nextVersion.current
    const branch = stateRef.current.branches[parentId]
    setState((previous) => ({
      ...previous,
      reads: { ...previous.reads, [parentId]: { status: "pending", controller } },
    }))

    const failed = (message: string) => setState((previous) => ({
      ...previous,
      reads: { ...previous.reads, [parentId]: { status: "failed", message } },
    }))

    try {
      const result = await loadComments(owner, sort, parentId || undefined, branch?.page?.next, levels, controller.signal)

      if (controller.signal.aborted) {
        return
      }

      if (!result.ok) {
        failed(result.error.errors?.[0]?.message || "Could not load comments.")
        return
      }

      setState((previous) => {
        const prefetched = result.data.replies || []
        const records = mergeRecords(previous, [...result.data.comments, ...prefetched], read)
        const connected = connectComments(records, prefetched)
        const next = acceptPermissions(connected, result.data, read)
        const incoming = result.data.comments.map((comment) => comment.id)
        const existing = previous.branches[parentId]?.ids || []
        const ids = branch?.page?.next
          ? [...new Set([...existing, ...incoming])]
          : [...new Set([...incoming, ...existing])]

        next.branches[parentId] = { ids, page: { next: result.data.next } }
        next.reads = { ...next.reads }
        delete next.reads[parentId]

        for (const reply of prefetched) {
          const parent = reply.parentId!
          next.branches[parent] = { ...next.branches[parent], page: {} }
        }

        return next
      })
    } catch (cause) {
      if (controller.signal.aborted) {
        return
      }

      if (!(cause instanceof RequestError)) {
        throw cause
      }

      failed("Could not load comments. Please try again.")
    }
  }, [owner.kind, owner.id, owner.number, sort, setState])

  useEffect(() => {
    if (recordRequest.current || recordsError || unread.length === 0) {
      return
    }

    const ids = unread
    const request = new AbortController()
    recordRequest.current = request
    const read = ++nextVersion.current
    const hydrate = async () => {
      try {
        const result = await loadCommentRecords(owner, ids, request.signal)

        if (request.signal.aborted) {
          return
        }

        if (!result.ok) {
          setRecordsError(result.error.errors?.[0]?.message || "Could not load comments.")
          return
        }

        setState((previous) => {
          const next = mergeRecords(previous, result.data.comments, read)
          const found = new Set(result.data.comments.map((comment) => comment.id))

          for (const id of ids) {
            const current = next.comments[id]

            if (!found.has(id) && current && "pending" in current) {
              next.comments[id] = { ...current, hasReplies: false, pending: "unavailable" }
            }

            const saved = previous.comments[id]
            const branch = next.branches[id]

            if (saved && "pending" in saved && !("pending" in current) && current.hasReplies && branch && branch.ids.length === 0) {
              next.branches = { ...next.branches, [id]: { ...branch, page: null } }
            }
          }

          return acceptPermissions(next, result.data, read)
        })
      } catch (cause) {
        if (request.signal.aborted) {
          return
        }

        if (!(cause instanceof RequestError)) {
          throw cause
        }

        setRecordsError("Could not load comments. Please try again.")
      } finally {
        if (recordRequest.current === request) {
          recordRequest.current = undefined
        }
      }
    }

    void hydrate()
  }, [unread, recordsError, owner.kind, owner.id, owner.number, sort])

  const visible = useCallback((ids: number[]) => {
    setVisibleRecords((previous) => {
      if (previous.length === ids.length && previous.every((id, index) => id === ids[index])) {
        return previous
      }

      return ids
    })
  }, [])

  useLayoutEffect(() => {
    if (!reading.saved) {
      return
    }

    if (recordsError) {
      setReadingTarget(undefined)
      reading.restored()
      return
    }

    const anchor = reading.saved.anchor
    const comment = anchor ? state.comments[anchor.id] : undefined

    if (!anchor || reading.cancelled.current || (comment && "pending" in comment && comment.pending === "unavailable")) {
      setReadingTarget(undefined)
      reading.restored()
      return
    }

    if (!comment || "pending" in comment || readingTarget === undefined) {
      return
    }

    if (visibleRecords.some((id) => {
      const visible = state.comments[id]
      return visible && "pending" in visible && visible.pending === "unloaded"
    })) {
      return
    }

    let frame = requestAnimationFrame(() => {
      frame = requestAnimationFrame(() => {
        if (!reading.cancelled.current) {
          const element = container.current?.querySelector<HTMLElement>(`#comment-${anchor.id}`)

          if (element) {
            window.scrollBy({ top: element.getBoundingClientRect().top - anchor.top, behavior: "instant" })
          }
        }

        setReadingTarget(undefined)
        reading.restored()
      })
    })

    return () => cancelAnimationFrame(frame)
  }, [state, visibleRecords, recordsError, readingTarget, reading.saved, reading.restored])

  const changeSort = (next: DiscussionSort) => {
    abortReads()
    setSort(next)
    setState((previous) => ({ ...previous, branches: {}, reads: {} }))
    setVisibleRecords([])
    setRecordsError(undefined)
    setReadingTarget(undefined)
    reading.restored()
    setContext(undefined)
  }

  const accept = useCallback((page: DiscussionPage, read: number) => {
    setState((previous) => {
      const records = mergeRecords(previous, page.comments, read)
      const connected = connectComments(records, page.comments)
      return acceptPermissions(connected, page, read)
    })
  }, [setState])

  const readContext = useCallback(async (id: number) => {
    contextRequest.current?.abort()
    const request = new AbortController()
    contextRequest.current = request
    const read = ++nextVersion.current
    setContextError(undefined)

    try {
      const result = await loadCommentContext(id, request.signal)

      if (request.signal.aborted) {
        return
      }

      if (!result.ok || result.data.owner.kind !== owner.kind || result.data.owner.id !== owner.id) {
        setContextError("This comment is unavailable.")
        return
      }

      accept(result.data, read)
      setContext({
        commentId: result.data.commentId,
        rootId: result.data.comments[0]?.id,
        nextAncestorId: result.data.nextAncestorId,
      })
      setCollapsed((previous) => {
        const next = { ...previous }

        for (const comment of result.data.comments) {
          next[comment.id] = false
        }

        return next
      })
    } catch (cause) {
      if (request.signal.aborted) {
        return
      }

      if (!(cause instanceof RequestError)) {
        throw cause
      }

      setContextError("Could not load this thread. Retry when the connection is available.")
    }
  }, [owner.kind, owner.id, sort, accept])

  useEffect(() => {
    const hashChanged = () => {
      const id = linkedComment()
      setTarget(id)

      if (id) {
        void readContext(id)
      } else {
        contextRequest.current?.abort()
        setContext(undefined)
        setContextError(undefined)
      }
    }

    hashChanged()
    window.addEventListener("hashchange", hashChanged)
    return () => {
      contextRequest.current?.abort()
      window.removeEventListener("hashchange", hashChanged)
    }
  }, [readContext])

  const rows = useMemo(
    () => discussionRows(state, collapsed, context?.rootId, expanded),
    [state.comments, state.branches, collapsed, context?.rootId, expanded]
  )
  const rowsRef = useRef(rows)
  rowsRef.current = rows

  const collapse = useCallback((id: number, value: boolean) => {
    setCollapsed((previous) => ({ ...previous, [id]: value }))
  }, [])

  const created = useCallback((comment: DiscussionComment) => {
    const existed = !!stateRef.current.comments[comment.id]
    const version = ++nextVersion.current
    setState((previous) => {
      const next = previous.comments[comment.id] ? previous : mergeRecords(previous, [comment], version)
      return connectComments(next, [comment])
    })

    if (!existed) {
      onCommentAdded?.()
      setEntering((previous) => new Set(previous).add(comment.id))

      const parent = rowsRef.current.find((row) => row.kind === "comment" && row.id === comment.parentId)

      if (parent?.kind === "comment" && parent.depth >= 4) {
        setExpanded((previous) => ({ ...previous, [parent.id]: true }))
      }
    }
  }, [onCommentAdded])

  const entered = useCallback((id: number) => {
    setEntering((previous) => {
      const next = new Set(previous)
      next.delete(id)
      return next
    })
  }, [])

  const changed = useCallback((comment: DiscussionComment, change: CommentChange) => {
    const version = ++nextVersion.current
    if (change === "report") {
      reportedComments.current.add(comment.id)
    }

    setState((previous) => {
      const current = previous.comments[comment.id]

      if (!current || "pending" in current || change === "delete") {
        return mergeRecords(previous, [comment], version)
      }

      if (current.state === "deleted") {
        return previous
      }

      let updated = current

      if (change === "edit") {
        if (!hasNewerEdit(comment, current)) {
          return previous
        }

        updated = {
          ...current,
          content: comment.content,
          attachments: comment.attachments,
          editedAt: comment.editedAt,
          editedBy: comment.editedBy,
        }
      } else if (change === "moderation") {
        updated = {
          ...current,
          state: comment.state,
          moderationPending: comment.moderationPending,
          permissions: {
            ...current.permissions,
            react: comment.permissions.react,
            report: comment.permissions.report && !reportedComments.current.has(comment.id),
          },
        }
      } else if (change === "report") {
        updated = { ...current, permissions: { ...current.permissions, report: false } }
      }

      return mergeRecords(previous, [updated], version)
    })
  }, [])

  const reactionsChanged = useCallback((id: number, reactions: ReactionCount[]) => {
    const version = ++nextVersion.current
    setState((previous) => {
      const comment = previous.comments[id]

      if (!comment || "pending" in comment || comment.state === "deleted") {
        return previous
      }

      return mergeRecords(previous, [{ ...comment, reactionCounts: reactions }], version)
    })
  }, [])

  const interact = useCallback((id: number, changes: Partial<CommentInteraction>) => {
    setInteractions((previous) => {
      const next = { ...previous }
      const value = { ...next[id], ...changes }

      if (value.editor || value.action) {
        next[id] = value
      } else {
        delete next[id]
      }

      return next
    })
  }, [])

  const edit = useCallback((id: number, editor?: "edit" | "reply") => {
    interact(id, { editor })
  }, [interact])

  const act = useCallback(async (id: number, operation: () => Promise<void>) => {
    if (activeActions.current.has(id)) {
      return
    }

    activeActions.current.add(id)
    interact(id, { action: { kind: "pending" } })

    try {
      await operation()
      interact(id, { action: undefined })
    } catch (cause) {
      interact(id, {
        action: {
          kind: "failed",
          error: { errors: [{ message: cause instanceof Error ? cause.message : "Could not complete this action." }], cause },
          retry: operation,
        },
      })
    } finally {
      activeActions.current.delete(id)
    }
  }, [interact])

  if (!state.permissions.comment && !state.permissions.signInToComment && rows.length === 0 && !target) {
    return null
  }

  return (
    <section ref={container} aria-label="Discussion" className="c-comment-list mt-8">
      <div className="flex items-center flex-wrap gap-3 mb-3">
        <h2 className="flex-1 text-lg font-semibold">Discussion</h2>
        {headerActions}
        <label className="flex items-center gap-2 text-sm">
          Sort by
          <select
            aria-label="Sort discussion"
            value={sort}
            onChange={(event) => changeSort(event.target.value as DiscussionSort)}
            className="cursor-pointer rounded border border-border bg-elevated px-2 py-1.5"
          >
            <option value="liked">Most liked</option>
            <option value="disliked">Most disliked</option>
            <option value="replies">Most replies</option>
            <option value="latest">Latest</option>
          </select>
        </label>
      </div>
      <SignInModal isOpen={signIn} onClose={() => setSignIn(false)} />
      {target && (
        <div className="flex gap-3 my-3">
          <a href={owner.url}>View full discussion</a>
          {context?.nextAncestorId && <a href={`#comment-${context.nextAncestorId}`}>View earlier replies</a>}
        </div>
      )}
      {contextError && (
        <div role="alert">
          <p>{contextError}</p>
          <Button onClick={() => target ? readContext(target) : undefined}>Retry thread</Button>
        </div>
      )}
      {recordsError && (
        <div role="alert" className="sticky top-14 z-10 my-3 flex items-center gap-3 rounded border border-border bg-surface p-3">
          <p className="flex-1 text-sm">{recordsError}</p>
          <Button onClick={() => setRecordsError(undefined)}>Retry loading comments</Button>
        </div>
      )}
      <DiscussionViewport
        key={`${sort}:${context?.commentId || 0}`}
        rows={rows}
        target={target}
        readingTarget={readingTarget}
        measurements={measurements.current}
        initialViewport={initialViewport.current}
        onVisible={visible}
        onCollapse={collapse}
      >
        {(row) => {
          if (row.kind === "continue") {
            return (
              <Button
                size="small"
                variant="tertiary"
                onClick={() => setExpanded((previous) => ({ ...previous, [row.parentId]: true }))}
              >
                Show more replies
              </Button>
            )
          }

          if (row.kind === "load") {
            return (
              <LoadComments
                key={`${sort}:${state.branches[row.parentId]?.page?.next || "first"}`}
                read={state.reads[row.parentId]}
                load={() => loadBranch(row.parentId, row.levels)}
              />
            )
          }

          const comment = state.comments[row.id]

          if ("pending" in comment) {
            return (
              <article
                id={`comment-${row.id}`}
                aria-busy={comment.pending === "unloaded" && !recordsError}
                aria-label={comment.pending === "unloaded" ? "Loading comment" : undefined}
                className="py-3 text-muted"
                style={{ minHeight: measurements.current.get(`comment:${row.id}`) ?? (row.collapsed ? 48 : 240) }}
              >
                {comment.pending === "unavailable" ? "Comment unavailable." : (
                  <div aria-hidden="true">
                    <div className="h-3 w-24 rounded bg-muted/15" />
                    <div className="mt-4 h-3 w-3/4 rounded bg-muted/10" />
                  </div>
                )}
              </article>
            )
          }

          return (
            <DiscussionCommentCard
              owner={owner}
              comment={comment}
              images={state.permissions.images}
              collapsed={row.collapsed}
              highlighted={row.id === target}
              entering={entering.has(row.id)}
              onEntered={entered}
              interaction={interactions[row.id]}
              onCreated={created}
              onChanged={changed}
              onReactionsChanged={reactionsChanged}
              onEditor={edit}
              onAction={act}
            />
          )
        }}
      </DiscussionViewport>
      {state.permissions.comment && <CommentComposer owner={owner} images={state.permissions.images} onSaved={created} />}
      {state.permissions.signInToComment && <Button onClick={() => setSignIn(true)}>Sign in to comment</Button>}
    </section>
  )
}
