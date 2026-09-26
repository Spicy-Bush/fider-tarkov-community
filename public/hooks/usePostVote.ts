import { useEffect, useRef, useState } from "react"
import { Post, PostStatus, isPostArchived, isPostLocked } from "@fider/models"
import { actions, analytics, notify } from "@fider/services"
import { useFider } from "@fider/hooks"
import { RequestError } from "@fider/services/http"

export function usePostVote(post: Post, onChange?: (upvotes: number, downvotes: number) => void) {
  const fider = useFider()
  const [isSignInModalOpen, setIsSignInModalOpen] = useState(false)
  const [vote, setVote] = useState({ direction: post.voteType, revision: post.voteRevision, upvotes: post.upvotes || 0, downvotes: post.downvotes || 0 })
  const [saving, setSaving] = useState(false)
  const inFlight = useRef(false)
  const status = PostStatus.Get(post.status)
  const isDisabled = saving || status.closed || fider.isReadOnly || isPostLocked(post)

  useEffect(() => {
    setVote({ direction: post.voteType, revision: post.voteRevision, upvotes: post.upvotes || 0, downvotes: post.downvotes || 0 })
  }, [post.id, post.voteType, post.voteRevision, post.upvotes, post.downvotes])

  const chooseVote = async (choice: "up" | "down") => {
    if (!fider.session.isAuthenticated) {
      setIsSignInModalOpen(true)
      return
    }
    if (isDisabled || inFlight.current) return
    inFlight.current = true
    setSaving(true)
    const requested = choice === "up" ? 1 : -1
    const desired = vote.direction === requested ? 0 : requested
    try {
      const result = await actions.setVote(post.number, desired, vote.revision)
      if (!result.ok) {
        notify.error(result.error.errors?.[0]?.message || "Your vote could not be saved.")
        return
      }
      const state = result.data
      setVote(state)
      if (state.direction === desired) {
        let eventAction = choice === "up" ? "upvote" : "downvote"
        if (desired === 0) {
          eventAction = "unvote"
        } else if (vote.direction !== 0) {
          eventAction = "toggle-vote"
        }
        analytics.event("post", eventAction)
      }
      onChange?.(state.upvotes, state.downvotes)
      if (!state.applied && state.direction !== desired) {
        notify.error("Your vote changed in another request. The current vote is shown.")
      }
      if (state.applied && isPostArchived(post)) location.reload()
    } catch (cause) {
      if (!(cause instanceof RequestError)) throw cause
      notify.error("Your vote has not been confirmed. Please try again.")
    } finally {
      inFlight.current = false
      setSaving(false)
    }
  }

  return {
    voteType: vote.direction === 1 ? "up" : vote.direction === -1 ? "down" : "none",
    upvotes: vote.upvotes,
    downvotes: vote.downvotes,
    status,
    isDisabled,
    isSignInModalOpen,
    closeSignInModal: () => setIsSignInModalOpen(false),
    chooseVote,
  }
}
