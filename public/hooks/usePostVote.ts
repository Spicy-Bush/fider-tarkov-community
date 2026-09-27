import { useEffect, useMemo, useState } from "react"
import { Post, PostStatus, isPostArchived } from "@fider/models"
import { actions, analytics, notify } from "@fider/services"
import { useFider } from "@fider/hooks"
import { RequestError } from "@fider/services/http"

export function usePostVote(post: Post, onChange?: (upvotes: number, downvotes: number) => void) {
  const fider = useFider()
  const tenantID = fider.session.tenant.id
  const userID = fider.session.isAuthenticated ? fider.session.user.id : undefined
  const [isSignInModalOpen, setIsSignInModalOpen] = useState(false)
  const owner = useMemo(() => ({
    attached: false,
    tenantID,
    userID,
    post,
    vote: voteFromPost(post),
    desired: null as { direction: number } | null,
    retry: null as { desired: { direction: number }; revision: number } | null,
    running: null as Promise<void> | null,
    onChange,
  }), [post.id, tenantID, userID])
  const [snapshot, setSnapshot] = useState(() => ({ owner, vote: owner.vote, desired: owner.desired }))
  const status = PostStatus.Get(post.status)
  const isDisabled = fider.session.isAuthenticated && !post.permissions.vote

  const publish = (current: typeof owner) => {
    if (current.attached) {
      setSnapshot({ owner: current, vote: current.vote, desired: current.desired })
    }
  }

  useEffect(() => {
    owner.post = post

    if (post.voteRevision >= owner.vote.revision) {
      owner.vote = voteFromPost(post)
    }

    publish(owner)
  }, [owner, post.number, post.status, post.voteType, post.voteRevision, post.upvotes, post.downvotes])

  useEffect(() => {
    owner.attached = true
    publish(owner)

    return () => {
      owner.attached = false
    }
  }, [owner])

  useEffect(() => {
    owner.onChange = onChange
  }, [owner, onChange])

  const chooseVote = async (choice: "up" | "down") => {
    if (!fider.session.isAuthenticated) {
      setIsSignInModalOpen(true)
      return
    }
    if (isDisabled) return

    const current = owner
    const requested = choice === "up" ? 1 : -1
    const shownDirection = current.desired?.direction ?? current.vote.direction
    current.desired = { direction: shownDirection === requested ? 0 : requested }
    publish(current)

    if (current.running) return current.running

    const drain = async () => {
      let restoredArchive = false

      try {
        while (current.desired) {
          const before = current.vote
          // Resolve uncertain writes before applying a later choice.
          const request = current.retry ?? { desired: current.desired, revision: before.revision }
          const desired = request.desired
          current.retry = request

          const result = await actions.setVote(current.post.number, desired.direction, request.revision)

          if (!fider.session.isAuthenticated || fider.session.user.id !== current.userID || fider.session.tenant.id !== current.tenantID) {
            current.retry = null
            return
          }

          if (!result.ok) {
            if (result.status && result.status < 500) {
              current.retry = null
            }

            if (current.attached) {
              notify.error(result.error.errors?.[0]?.message || "Your vote could not be saved.")
            }

            return
          }

          current.retry = null
          restoredArchive ||= result.data.applied && isPostArchived(current.post)
          const acknowledged = result.data.revision >= current.vote.revision

          if (acknowledged) {
            current.vote = result.data
          }

          const state = current.vote
          if (acknowledged && state.direction === desired.direction) {
            let event = desired.direction === 1 ? "upvote" : "downvote"

            if (desired.direction === 0) {
              event = "unvote"
            } else if (before.direction !== 0) {
              event = "toggle-vote"
            }

            analytics.event("post", event)
          }

          if (current.attached) {
            current.onChange?.(state.upvotes, state.downvotes)
          }

          if (current.desired === desired || current.desired.direction === state.direction) {
            if (current.desired === desired && state.direction !== desired.direction && current.attached) {
              notify.error("Your vote changed in another request. The current vote is shown.")
            }

            return
          }

          publish(current)
        }
      } catch (cause) {
        if (!(cause instanceof RequestError)) {
          current.retry = null
          throw cause
        }

        if (current.attached) {
          notify.error("Your vote has not been confirmed. Please try again.")
        }
      } finally {
        current.running = null
        current.desired = null
        publish(current)

        if (restoredArchive && current.attached) {
          location.reload()
        }
      }
    }

    current.running = drain()
    return current.running
  }

  const visible = snapshot.owner === owner ? snapshot : owner
  const shown = visible.desired ? withDirection(visible.vote, visible.desired.direction) : visible.vote

  return {
    voteType: shown.direction === 1 ? "up" : shown.direction === -1 ? "down" : "none",
    upvotes: shown.upvotes,
    downvotes: shown.downvotes,
    status,
    isDisabled,
    isSaving: visible.desired !== null,
    isSignInModalOpen,
    closeSignInModal: () => setIsSignInModalOpen(false),
    chooseVote,
  }
}

function voteFromPost(post: Post) {
  return {
    direction: post.voteType,
    revision: post.voteRevision,
    upvotes: post.upvotes || 0,
    downvotes: post.downvotes || 0,
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
