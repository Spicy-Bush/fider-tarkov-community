import React, { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { Button, SignInModal } from "@fider/components"
import { useFider } from "@fider/hooks"
import { CommentContext, DiscussionComment, DiscussionOwner, DiscussionPage, DiscussionPermissions, DiscussionSort, ReactionCount } from "@fider/models"
import { isNegativelyRated, loadCommentContext, loadComments } from "@fider/services/discussion"
import { CommentComposer } from "./CommentComposer"
import { CommentChange, CommentInteraction, DiscussionCommentCard } from "./DiscussionCommentCard"
import { DiscussionRow, DiscussionViewport } from "./DiscussionViewport"

interface Branch {
  ids: number[]
  loaded: boolean
  next?: string
  error?: string
}

interface DiscussionState {
  comments: Record<number, DiscussionComment>
  branches: Record<number, Branch>
  permissions?: DiscussionPermissions
}

interface DiscussionProps {
  owner: DiscussionOwner
  onCommentAdded?: () => void
  headerActions?: React.ReactNode
}

function mergeRecords(
  state: DiscussionState,
  comments: DiscussionComment[],
  beforeRead?: DiscussionState["comments"]
): DiscussionState {
  const next = { ...state, comments: { ...state.comments } }

  for (const comment of comments) {
    if (!beforeRead || !state.comments[comment.id] || state.comments[comment.id] === beforeRead[comment.id]) {
      next.comments[comment.id] = comment
    }
  }

  return next
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
    const branch = branches[parent] || { ids: [], loaded: false }
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
  state: DiscussionState,
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

      if (!branch?.loaded || branch.next) {
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
    const closed = collapsed[row.id] ?? isNegativelyRated(comment)
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

function LoadComments(props: { branch?: Branch; load: () => Promise<void> }) {
  const ref = useRef<HTMLDivElement>(null)
  const [loading, setLoading] = useState(false)
  const load = async () => {
    if (loading) {
      return
    }

    setLoading(true)

    try {
      await props.load()
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (!ref.current || props.branch?.error || loading) {
      return
    }

    const observer = new IntersectionObserver((entries) => {
      if (entries.some((entry) => entry.isIntersecting)) {
        void load()
      }
    }, { rootMargin: "300px" })

    observer.observe(ref.current)
    return () => observer.disconnect()
  }, [props.load, props.branch?.error, loading])

  return (
    <div ref={ref} className="py-3">
      {props.branch?.error && <p role="alert" className="text-danger">{props.branch.error}</p>}
      <Button size="small" loading={loading} onClick={load}>
        {props.branch?.error ? "Retry loading comments" : "Load more comments"}
      </Button>
    </div>
  )
}

export function Discussion(props: DiscussionProps) {
  const [sort, setSort] = useState<DiscussionSort>("liked")

  return (
    <section aria-label="Discussion" className="mt-4">
      <div className="flex items-center flex-wrap gap-3 mb-3">
        <h2 className="flex-1 text-lg font-semibold">Discussion</h2>
        {props.headerActions}
        <label className="flex items-center gap-2 text-sm">
          Sort by
          <select
            aria-label="Sort discussion"
            value={sort}
            onChange={(event) => setSort(event.target.value as DiscussionSort)}
            className="cursor-pointer rounded border border-border bg-elevated px-2 py-1.5"
          >
            <option value="liked">Most liked</option>
            <option value="disliked">Most disliked</option>
            <option value="replies">Most replies</option>
            <option value="latest">Latest</option>
          </select>
        </label>
      </div>
      <DiscussionThreads key={`${props.owner.kind}:${props.owner.id}`} {...props} sort={sort} />
    </section>
  )
}

function DiscussionThreads({ owner, onCommentAdded, sort }: DiscussionProps & { sort: DiscussionSort }) {
  const fider = useFider()
  const [state, setState] = useState<DiscussionState>({ comments: {}, branches: {} })
  const [collapsed, setCollapsed] = useState<Record<number, boolean>>({})
  const [expanded, setExpanded] = useState<Record<number, boolean>>({})
  const [context, setContext] = useState<CommentContext>()
  const [target, setTarget] = useState<number>()
  const [contextError, setContextError] = useState<string>()
  const [signIn, setSignIn] = useState(false)
  const [interactions, setInteractions] = useState<Record<number, CommentInteraction>>({})
  const activeActions = useRef(new Set<number>())
  const reportedComments = useRef(new Set<number>())
  const activeLoads = useRef(new Set<number>())
  const branchGeneration = useRef(0)
  const contextRequest = useRef(0)
  const stateRef = useRef(state)
  stateRef.current = state

  useEffect(() => {
    branchGeneration.current++
    activeLoads.current.clear()
    setState((previous) => ({ ...previous, branches: {} }))
    setContext(undefined)
  }, [sort])

  const accept = useCallback((page: DiscussionPage, beforeRead?: DiscussionState["comments"]) => {
    setState((previous) => ({
      ...connectComments(mergeRecords(previous, page.comments, beforeRead), page.comments),
      permissions: page.permissions,
    }))
  }, [])

  const loadBranch = useCallback(async (parentId: number, levels: number) => {
    if (activeLoads.current.has(parentId)) {
      return
    }

    activeLoads.current.add(parentId)
    const generation = branchGeneration.current
    const beforeRead = stateRef.current
    const branch = beforeRead.branches[parentId]

    try {
      const result = await loadComments(owner, sort, parentId || undefined, branch?.next, levels)

      if (generation !== branchGeneration.current) {
        return
      }

      if (!result.ok) {
        throw new Error(result.error.errors?.[0]?.message || "Could not load comments.")
      }

      setState((previous) => {
        const prefetched = result.data.replies || []
        const next = connectComments(mergeRecords(previous, [...result.data.comments, ...prefetched], beforeRead.comments), prefetched)
        next.permissions = result.data.permissions
        const incoming = result.data.comments.map((comment) => comment.id)
        const existing = previous.branches[parentId]?.ids || []
        const ids = branch?.loaded
          ? [...new Set([...existing, ...incoming])]
          : [...new Set([...incoming, ...existing])]

        next.branches[parentId] = {
          ids,
          loaded: true,
          next: result.data.next,
        }

        for (const reply of prefetched) {
          const parent = reply.parentId!
          next.branches[parent] = { ...next.branches[parent], loaded: true }
        }

        return next
      })
    } catch (cause) {
      if (generation !== branchGeneration.current) {
        return
      }

      setState((previous) => ({
        ...previous,
        branches: {
          ...previous.branches,
          [parentId]: {
            ...(previous.branches[parentId] || { ids: [], loaded: false }),
            error: cause instanceof Error ? cause.message : "Could not load comments.",
          },
        },
      }))
    } finally {
      if (generation === branchGeneration.current) {
        activeLoads.current.delete(parentId)
      }
    }
  }, [owner.kind, owner.id, owner.number, sort])

  const readContext = useCallback(async (id: number) => {
    const request = ++contextRequest.current
    const beforeRead = stateRef.current.comments
    setContextError(undefined)

    try {
      const result = await loadCommentContext(id)

      if (request !== contextRequest.current) {
        return
      }

      if (!result.ok || result.data.owner.kind !== owner.kind || result.data.owner.id !== owner.id) {
        setContextError("This comment is unavailable.")
        return
      }

      accept(result.data, beforeRead)
      setContext(result.data)
      setCollapsed((previous) => {
        const next = { ...previous }

        for (const comment of result.data.comments) {
          next[comment.id] = false
        }

        return next
      })
    } catch {
      if (request === contextRequest.current) {
        setContextError("Could not load this thread. Retry when the connection is available.")
      }
    }
  }, [owner.kind, owner.id, sort, accept])

  useEffect(() => {
    const hashChanged = () => {
      const match = /^#comment-(\d+)$/.exec(window.location.hash)
      const id = match ? Number(match[1]) : undefined
      setTarget(id)

      if (id) {
        void readContext(id)
      } else {
        contextRequest.current++
        setContext(undefined)
        setContextError(undefined)
      }
    }

    hashChanged()
    window.addEventListener("hashchange", hashChanged)
    return () => {
      contextRequest.current++
      window.removeEventListener("hashchange", hashChanged)
    }
  }, [readContext])

  const rows = useMemo(
    () => discussionRows(state, collapsed, context?.comments[0]?.id, expanded),
    [state, collapsed, context, expanded]
  )
  const rowsRef = useRef(rows)
  rowsRef.current = rows

  const collapse = useCallback((id: number, value: boolean) => {
    setCollapsed((previous) => ({ ...previous, [id]: value }))
  }, [])

  const created = useCallback((comment: DiscussionComment) => {
    const existed = !!stateRef.current.comments[comment.id]
    setState((previous) => {
      const next = previous.comments[comment.id] ? previous : mergeRecords(previous, [comment])
      return connectComments(next, [comment])
    })

    if (!existed) {
      onCommentAdded?.()

      const parent = rowsRef.current.find((row) => row.kind === "comment" && row.id === comment.parentId)

      if (parent?.kind === "comment" && parent.depth >= 4) {
        setExpanded((previous) => ({ ...previous, [parent.id]: true }))
      }
    }
  }, [onCommentAdded])

  const changed = useCallback((comment: DiscussionComment, change: CommentChange) => {
    if (change === "report") {
      reportedComments.current.add(comment.id)
    }

    setState((previous) => {
      const current = previous.comments[comment.id]

      if (!current || change === "delete") {
        return mergeRecords(previous, [comment])
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

      return mergeRecords(previous, [updated])
    })
  }, [])

  const reactionsChanged = useCallback((id: number, reactions: ReactionCount[]) => {
    setState((previous) => {
      const comment = previous.comments[id]

      if (!comment || comment.state === "deleted") {
        return previous
      }

      return mergeRecords(previous, [{ ...comment, reactionCounts: reactions }])
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

  return (
    <div className="c-comment-list">
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
      <DiscussionViewport key={`${sort}:${context?.commentId || 0}`} rows={rows} target={target} onCollapse={collapse}>
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
                key={`${sort}:${state.branches[row.parentId]?.next || "first"}`}
                branch={state.branches[row.parentId]}
                load={() => loadBranch(row.parentId, row.levels)}
              />
            )
          }

          return (
            <DiscussionCommentCard
              owner={owner}
              comment={state.comments[row.id]}
              images={!!state.permissions?.images}
              collapsed={row.collapsed}
              highlighted={row.id === target}
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
      {state.permissions?.comment && <CommentComposer owner={owner} images={state.permissions.images} onSaved={created} />}
      {state.permissions && !state.permissions.comment && (
        fider.session.isAuthenticated
          ? <p className="text-muted my-4">New comments are unavailable here.</p>
          : <Button onClick={() => setSignIn(true)}>Sign in to comment</Button>
      )}
    </div>
  )
}
