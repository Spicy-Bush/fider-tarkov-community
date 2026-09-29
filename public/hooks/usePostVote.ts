import { useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from "react"
import { Post } from "@fider/models"
import { PostStatus, isPostArchived } from "@fider/models/post"
import { UserStatus } from "@fider/models/identity"
import * as postActions from "@fider/services/actions/post"
import * as notify from "@fider/services/notify"
import { analytics } from "@fider/services/analytics"
import { useFider } from "@fider/hooks/use-fider"
import { RequestError } from "@fider/services/http"

interface VoteSnapshot {
  vote: ReturnType<typeof voteFromPost>
  desired: { direction: number } | null
}

interface VoteOwner {
  post: Post
  snapshot: VoteSnapshot
  retry: { desired: { direction: number }; revision: number; uncertain: boolean } | null
  running: Promise<void> | null
  listeners: Map<() => void, boolean>
}

// Only mounted controls and unfinished requests retain a post record.
const sessions = new WeakMap<object, Map<string, VoteOwner>>()

export function usePostVote(post: Post) {
  const fider = useFider()
  const session = fider.session
  const tenantID = session.tenant.id
  const userID = session.isAuthenticated ? session.user.id : undefined
  const key = `${tenantID}:${userID ?? "anonymous"}:${post.id}`
  const [isSignInModalOpen, setIsSignInModalOpen] = useState(false)
  const listener = useRef<(() => void) | undefined>(undefined)
  const initial = useMemo<VoteSnapshot>(() => ({ vote: voteFromPost(post), desired: null }), [
    key, post.voteType, post.voteRevision, post.upvotes, post.downvotes, post.lastActivityAt,
  ])

  const subscribe = useCallback((render: () => void) => {
    let records = sessions.get(session)
    if (!records) {
      records = new Map()
      sessions.set(session, records)
    }

    let owner = records.get(key)
    if (!owner) {
      owner = { post, snapshot: initial, retry: null, running: null, listeners: new Map() }
      records.set(key, owner)
    }

    listener.current = render
    owner.listeners.set(render, post.permissions.vote)
    return () => {
      owner.listeners.delete(render)
      release(records, key, owner)
    }
  }, [session, key])

  const getSnapshot = useCallback(() => sessions.get(session)?.get(key)?.snapshot ?? initial, [session, key, initial])
  const snapshot = useSyncExternalStore(subscribe, getSnapshot, () => initial)

  useEffect(() => {
    const owner = sessions.get(session)?.get(key)
    if (!owner) {
      return
    }

    owner.post = post
    if (listener.current) {
      owner.listeners.set(listener.current, post.permissions.vote)
    }

    owner.snapshot = {
      ...owner.snapshot,
      vote: post.voteRevision >= owner.snapshot.vote.revision ? voteFromPost(post) : owner.snapshot.vote,
    }
    publish(owner)
  }, [session, key, initial, post.number, post.status, post.permissions.vote])

  const chooseVote = async (choice: "up" | "down") => {
    if (!session.isAuthenticated) {
      setIsSignInModalOpen(true)
      return
    }

    const records = sessions.get(session)
    const owner = records?.get(key)
    if (!records || !owner || !currentAccount() || !canVote(owner)) {
      return
    }

    const requested = choice === "up" ? 1 : -1
    const shownDirection = owner.snapshot.desired?.direction ?? owner.snapshot.vote.direction
    owner.snapshot = { ...owner.snapshot, desired: { direction: shownDirection === requested ? 0 : requested } }
    publish(owner)

    if (owner.running) {
      return owner.running
    }

    const drain = async () => {
      let restoredArchive = false

      try {
        while (owner.snapshot.desired && currentAccount() && canVote(owner)) {
          const before = owner.snapshot.vote
          // Resolve uncertain writes before applying a later choice.
          const request = owner.retry ?? { desired: owner.snapshot.desired, revision: before.revision, uncertain: false }
          const desired = request.desired
          owner.retry = request
          const result = await postActions.setVote(owner.post.number, desired.direction, request.revision)

          if (!currentAccount()) {
            owner.retry = null
            return
          }
          if (!result.ok) {
            request.uncertain ||= result.unconfirmed
            if (!request.uncertain) {
              owner.retry = null
            }
            if (owner.listeners.size) {
              notify.error(result.error.errors?.[0]?.message || "Your vote could not be saved.")
            }
            return
          }

          owner.retry = null
          restoredArchive ||= result.data.applied && isPostArchived(owner.post)
          const acknowledged = result.data.revision >= owner.snapshot.vote.revision
          if (acknowledged) {
            owner.snapshot = { ...owner.snapshot, vote: result.data }
          }

          const state = owner.snapshot.vote
          if (acknowledged && state.direction === desired.direction) {
            let event = desired.direction === 1 ? "upvote" : "downvote"
            if (desired.direction === 0) {
              event = "unvote"
            } else if (before.direction !== 0) {
              event = "toggle-vote"
            }
            analytics.event("post", event)
          }

          if (owner.snapshot.desired === desired || owner.snapshot.desired?.direction === state.direction) {
            if (owner.snapshot.desired === desired && state.direction !== desired.direction && owner.listeners.size) {
              notify.error("Your vote changed in another request. The current vote is shown.")
            }
            return
          }
          publish(owner)
        }
      } finally {
        owner.running = null
        owner.snapshot = { ...owner.snapshot, desired: null }
        publish(owner)
        release(records, key, owner)
        if (restoredArchive && owner.listeners.size && currentAccount()) {
          location.reload()
        }
      }
    }

    owner.running = drain()
    return owner.running
  }

  function currentAccount() {
    return fider.session === session && session.isAuthenticated && session.user.id === userID && session.tenant.id === tenantID
  }

  function canVote(owner?: VoteOwner) {
    return !fider.isReadOnly && !session.user.isMuted && session.user.status !== UserStatus.Blocked && allowed(owner)
  }

  const refresh = useCallback(async () => {
    try {
      const result = await postActions.getPost(post.number)
      const owner = sessions.get(session)?.get(key)
      if (result.ok && owner && currentAccount() && result.data.voteRevision >= owner.snapshot.vote.revision) {
        owner.post = result.data
        owner.snapshot = { ...owner.snapshot, vote: voteFromPost(result.data) }
        publish(owner)
      }
    } catch (cause) {
      if (!(cause instanceof RequestError)) {
        throw cause
      }
      if (currentAccount()) {
        notify.error("Post activity could not be refreshed. Please reload the page.")
      }
    }
  }, [fider, session, key, post.number])

  const shown = snapshot.desired ? withDirection(snapshot.vote, snapshot.desired.direction) : snapshot.vote
  return {
    voteType: shown.direction === 1 ? "up" : shown.direction === -1 ? "down" : "none",
    upvotes: shown.upvotes,
    downvotes: shown.downvotes,
    lastActivityAt: snapshot.vote.lastActivityAt,
    refresh,
    status: PostStatus.Get(post.status),
    isDisabled: session.isAuthenticated && (!post.permissions.vote || !canVote(sessions.get(session)?.get(key))),
    isSaving: snapshot.desired !== null,
    isSignInModalOpen,
    closeSignInModal: () => setIsSignInModalOpen(false),
    chooseVote,
  }
}

function allowed(owner?: VoteOwner) {
  if (!owner) {
    return true
  }
  if (!owner.post.permissions.vote) {
    return false
  }
  for (const permission of owner.listeners.values()) {
    if (!permission) {
      return false
    }
  }
  return true
}

function publish(owner: VoteOwner) {
  for (const listener of owner.listeners.keys()) {
    listener()
  }
}

function release(records: Map<string, VoteOwner>, key: string, owner: VoteOwner) {
  if (!owner.listeners.size && !owner.running && records.get(key) === owner) {
    records.delete(key)
  }
}

function voteFromPost(post: Post) {
  return {
    direction: post.voteType,
    revision: post.voteRevision,
    upvotes: post.upvotes || 0,
    downvotes: post.downvotes || 0,
    lastActivityAt: post.lastActivityAt,
  }
}

function withDirection<T extends { direction: number; upvotes: number; downvotes: number }>(vote: T, direction: number): T {
  return {
    ...vote,
    direction,
    upvotes: vote.upvotes - (vote.direction === 1 ? 1 : 0) + (direction === 1 ? 1 : 0),
    downvotes: vote.downvotes - (vote.direction === -1 ? 1 : 0) + (direction === -1 ? 1 : 0),
  }
}
