import React, { useState, useCallback, useEffect, useRef } from "react"
import ReactDOM from "react-dom"
import { Post, Tag, PostStatus, isPostLocked, Comment } from "@fider/models"
import { actions, PAGINATION } from "@fider/services"
import { Icon, Button, SignInModal } from "@fider/components"
import { VStack, HStack } from "@fider/components/layout"
import { SwipeCard } from "./SwipeCard"
import { useFider, useStackNavigation } from "@fider/hooks"
import {
  heroiconsX as IconX,
  heroiconsThumbsup as IconUp,
  heroiconsThumbsdown as IconDown,
  heroiconsSelector as IconHand,
  swipeCards as IconSwipeCards,
} from "@fider/icons.generated"
import { Trans } from "@lingui/react/macro"

type ControlMode = "swipe" | "buttons"

interface SwipeModeProps {
  tags: Tag[]
  isOpen: boolean
  onClose: () => void
}

interface SwipeState extends Record<string, unknown> {
  swipeIndex: number
}

function useLatest<T>(value: T): React.MutableRefObject<T> {
  const ref = useRef(value)
  ref.current = value
  return ref
}

function canVoteOnPost(post: Post, isReadOnly: boolean): boolean {
  const status = PostStatus.Get(post.status)
  const locked = isPostLocked(post) || post.lockedSettings?.locked
  return !status.closed && !isReadOnly && !locked
}

function filterVotablePosts(posts: Post[], isReadOnly: boolean): Post[] {
  return posts.filter(p => p.voteType === 0 && canVoteOnPost(p, isReadOnly))
}

export const SwipeMode: React.FC<SwipeModeProps> = ({ tags, isOpen, onClose }) => {
  const fider = useFider()
  const root = useRef<HTMLElement | null>(null)
  const [showOnboarding, setShowOnboarding] = useState(true)
  const [controlMode, setControlMode] = useState<ControlMode>("swipe")
  const [posts, setPosts] = useState<Post[]>([])
  const [currentIndex, setCurrentIndex] = useState(0)
  const [voteHistory, setVoteHistory] = useState<{ post: Post; newVote: "up" | "down" }[]>([])
  const [isSignInModalOpen, setIsSignInModalOpen] = useState(false)
  const [attachmentsMap, setAttachmentsMap] = useState<Record<number, string[]>>({})
  const [commentsMap, setCommentsMap] = useState<Record<number, Comment[]>>({})
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [hasMore, setHasMore] = useState(true)
  const [swipeProgress, setSwipeProgress] = useState(0)
  const [swipeDirection, setSwipeDirection] = useState<"left" | "right" | null>(null)

  const postsRef = useLatest(posts)
  const currentIndexRef = useLatest(currentIndex)
  const onCloseRef = useLatest(onClose)
  const offsetRef = useRef(0)
  const seenPostIds = useRef(new Set<number>())
  const hasInitializedRef = useRef(false)
  const loadedPostData = useRef(new Set<number>())

  const handleStateChange = useCallback((state: SwipeState | null) => {
    if (state && typeof state.swipeIndex === "number") {
      setCurrentIndex(state.swipeIndex)
    } else {
      onCloseRef.current()
    }
  }, [])

  const { pushState, isNavigating } = useStackNavigation<SwipeState>({
    onStateChange: handleStateChange,
  })

  useEffect(() => {
    root.current = document.getElementById("root-modal")
  }, [])

  const loadPosts = useCallback(async (reset: boolean) => {
    setIsLoadingMore(true)
    if (reset) {
      offsetRef.current = 0
      seenPostIds.current.clear()
    }
    const response = await actions.searchPosts({
      view: "trending",
      notMyVotes: true,
      limit: PAGINATION.DEFAULT_LIMIT,
      offset: offsetRef.current,
    })
    if (response.ok && response.data) {
      const newPosts = response.data
      setHasMore(newPosts.length === PAGINATION.DEFAULT_LIMIT)
      const uniquePosts: Post[] = []
      for (const post of newPosts) {
        if (!seenPostIds.current.has(post.id)) {
          seenPostIds.current.add(post.id)
          uniquePosts.push(post)
        }
      }
      const votable = filterVotablePosts(uniquePosts, fider.isReadOnly)
      if (reset) {
        setPosts(votable)
        setCurrentIndex(0)
        setVoteHistory([])
      } else {
        setPosts(prev => [...prev, ...votable])
      }
      offsetRef.current += newPosts.length
    }
    setIsLoadingMore(false)
  }, [fider.isReadOnly])

  useEffect(() => {
    if (isOpen) {
      setShowOnboarding(true)
      document.body.style.overflow = "hidden"
      if (!hasInitializedRef.current) {
        hasInitializedRef.current = true
        setAttachmentsMap({})
        setCommentsMap({})
        loadedPostData.current.clear()
        pushState({ swipeIndex: 0 })
        loadPosts(true)
      }
    } else {
      document.body.style.overflow = ""
      hasInitializedRef.current = false
    }
    return () => { document.body.style.overflow = "" }
  }, [isOpen, loadPosts, pushState])

  useEffect(() => {
    const currentPost = posts[currentIndex]
    if (!isOpen || !currentPost || loadedPostData.current.has(currentPost.number)) return
    loadedPostData.current.add(currentPost.number)

    actions.getPostAttachments(currentPost.number).then(result => {
      if (result.ok && result.data) {
        setAttachmentsMap(prev => ({ ...prev, [currentPost.number]: result.data! }))
      }
    })
    actions.getAllComments(currentPost.number).then(result => {
      if (result.ok && result.data) {
        setCommentsMap(prev => ({ ...prev, [currentPost.number]: result.data! }))
      }
    })
  }, [isOpen, currentIndex, posts])

  useEffect(() => {
    if (isOpen && hasMore && !isLoadingMore && posts.length > 0 && currentIndex >= posts.length - 3) {
      loadPosts(false)
    }
  }, [isOpen, currentIndex, posts.length, hasMore, isLoadingMore, loadPosts])

  const handleVote = useCallback(async (direction: "left" | "right") => {
    if (isNavigating.current) return
    if (!fider.session.isAuthenticated) {
      setIsSignInModalOpen(true)
      return
    }
    const currentPost = postsRef.current[currentIndexRef.current]
    if (!currentPost) return

    const voteType = direction === "right" ? "up" : "down"
    const newIndex = currentIndexRef.current + 1

    setVoteHistory(prev => [...prev, { post: currentPost, newVote: voteType }])
    setCurrentIndex(newIndex)
    pushState({ swipeIndex: newIndex })

    if (voteType === "up") {
      actions.addVote(currentPost.number)
    } else {
      actions.addDownVote(currentPost.number)
    }
  }, [fider.session.isAuthenticated, pushState])

  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    if (!isOpen || showOnboarding) return
    if (e.key === "Escape") onClose()
    else if (e.key === "ArrowLeft") handleVote("left")
    else if (e.key === "ArrowRight") handleVote("right")
  }, [isOpen, showOnboarding, onClose, handleVote])

  useEffect(() => {
    window.addEventListener("keydown", handleKeyDown)
    return () => window.removeEventListener("keydown", handleKeyDown)
  }, [handleKeyDown])

  if (!isOpen || !root.current) return null

  const currentPost = posts[currentIndex]
  const nextPost = posts[currentIndex + 1]
  const isComplete = currentIndex >= posts.length

  const content = (
    <div className="fixed inset-0 z-toolbar bg-elevated flex flex-col">
      <div className="fixed bottom-5 right-5 flex gap-2 z-sidebar">
        <button 
          className="flex items-center justify-center w-11 h-11 rounded-full border border-border bg-elevated text-muted cursor-pointer shadow-lg hover:bg-surface-alt"
          onClick={onClose}
        >
          <Icon sprite={IconX} className="h-5 w-5" />
        </button>
      </div>

      {controlMode === "buttons" && !showOnboarding && !isComplete && (
        <div className="fixed bottom-5 left-1/2 -translate-x-1/2 flex gap-8 z-511">
          <button 
            className="flex items-center justify-center w-16 h-16 rounded-full border-none cursor-pointer shadow-xl bg-danger text-white active:scale-95"
            onClick={() => handleVote("left")}
          >
            <Icon sprite={IconDown} className="h-8 w-8" />
          </button>
          <button 
            className="flex items-center justify-center w-16 h-16 rounded-full border-none cursor-pointer shadow-xl bg-success text-white active:scale-95"
            onClick={() => handleVote("right")}
          >
            <Icon sprite={IconUp} className="h-8 w-8" />
          </button>
        </div>
      )}

      {controlMode === "swipe" && !showOnboarding && !isComplete && swipeProgress > 0 && (
        <div className="fixed top-20 left-1/2 -translate-x-1/2 z-512 pointer-events-none">
          {swipeDirection === "right" && (
            <div 
              className="flex flex-col items-center gap-1 px-6 py-4 rounded-2xl font-bold text-sm uppercase tracking-wider bg-success/95 text-white shadow-lg"
              style={{ opacity: swipeProgress }}
            >
              <Icon sprite={IconUp} className="h-12 w-12" />
              <span><Trans id="action.upvote">Upvote</Trans></span>
            </div>
          )}
          {swipeDirection === "left" && (
            <div 
              className="flex flex-col items-center gap-1 px-6 py-4 rounded-2xl font-bold text-sm uppercase tracking-wider bg-danger/95 text-white shadow-lg"
              style={{ opacity: swipeProgress }}
            >
              <Icon sprite={IconDown} className="h-12 w-12" />
              <span><Trans id="action.downvote">Downvote</Trans></span>
            </div>
          )}
        </div>
      )}

      <div className="flex-1 flex flex-col overflow-hidden">
        {showOnboarding ? (
          <OnboardingScreen onComplete={(mode) => { setControlMode(mode); setShowOnboarding(false) }} />
        ) : isComplete ? (
          <CompleteScreen onClose={onClose} votedCount={voteHistory.length} />
        ) : (
          <div className="relative flex-1">
            {nextPost && (
              <SwipeCard post={nextPost} tags={tags} isActive={false} controlMode={controlMode} style={{ opacity: 0.3 }} />
            )}
            {currentPost && (
              <SwipeCard
                key={currentPost.id}
                post={currentPost}
                tags={tags}
                attachments={attachmentsMap[currentPost.number]}
                comments={commentsMap[currentPost.number]}
                isActive={true}
                controlMode={controlMode}
                onSwipe={handleVote}
                onOpenPost={() => window.open(`/posts/${currentPost.number}/${currentPost.slug}`, "_blank")}
                onSwipeProgress={(p, d) => { setSwipeProgress(p); setSwipeDirection(d) }}
              />
            )}
          </div>
        )}
      </div>

      <SignInModal isOpen={isSignInModalOpen} onClose={() => setIsSignInModalOpen(false)} />
    </div>
  )

  return ReactDOM.createPortal(content, root.current)
}

const OnboardingScreen: React.FC<{ onComplete: (mode: ControlMode) => void }> = ({ onComplete }) => (
  <div className="flex items-center justify-center flex-1 p-6">
    <div className="flex flex-col items-center justify-center text-center w-full h-full">
      <Icon sprite={IconSwipeCards} className="w-[72px] h-[72px] mb-5" />
      <h2 className="text-[28px] font-semibold text-foreground m-0 mb-3">
        <Trans id="swipemode.onboarding.title">Swipe Mode</Trans>
      </h2>
      <p className="text-base text-muted m-0">
        <Trans id="swipemode.onboarding.description">Quickly vote on posts by swiping through them.</Trans>
      </p>
      <div className="flex items-center justify-center gap-6 my-8">
        <div className="flex flex-col items-center gap-2 text-sm font-semibold text-danger">
          <Icon sprite={IconDown} className="h-6 w-6" />
          <span><Trans id="action.downvote">Downvote</Trans></span>
        </div>
        <div className="w-20 h-[100px] bg-surface-alt rounded-panel flex items-center justify-center font-medium text-muted border border-border text-sm">
          <Trans id="swipemode.onboarding.post">Post</Trans>
        </div>
        <div className="flex flex-col items-center gap-2 text-sm font-semibold text-success">
          <Icon sprite={IconUp} className="h-6 w-6" />
          <span><Trans id="action.upvote">Upvote</Trans></span>
        </div>
      </div>
      <p className="text-lg font-semibold text-foreground mt-8 mb-4">
        <Trans id="swipemode.onboarding.chooseinput">How do you want to vote?</Trans>
      </p>
      <VStack spacing={2} className="w-full max-w-[320px]">
        <button 
          className="flex items-center gap-4 w-full px-5 py-4 border border-surface-alt rounded-panel bg-elevated cursor-pointer text-left hover:border-primary hover:bg-tertiary [&>svg]:shrink-0 [&>svg]:text-primary [&>svg]:rotate-90"
          onClick={() => onComplete("swipe")}
        >
          <Icon sprite={IconHand} className="h-5 w-5" />
          <div className="flex flex-col gap-0.5">
            <strong className="text-base text-foreground"><Trans id="swipemode.option.swipe.title">Swipe Gestures</Trans></strong>
            <span className="text-xs text-muted"><Trans id="swipemode.option.swipe.description">Swipe left or right</Trans></span>
          </div>
        </button>
        <button 
          className="flex items-center gap-4 w-full px-5 py-4 border border-surface-alt rounded-panel bg-elevated cursor-pointer text-left hover:border-primary hover:bg-tertiary [&>div:first-child]:shrink-0 [&>div:first-child]:text-primary"
          onClick={() => onComplete("buttons")}
        >
          <HStack spacing={1}>
            <Icon sprite={IconDown} className="h-4 w-4" />
            <Icon sprite={IconUp} className="h-4 w-4" />
          </HStack>
          <div className="flex flex-col gap-0.5">
            <strong className="text-base text-foreground"><Trans id="swipemode.option.buttons.title">Floating Buttons</Trans></strong>
            <span className="text-xs text-muted"><Trans id="swipemode.option.buttons.description">Tap buttons to vote</Trans></span>
          </div>
        </button>
      </VStack>
    </div>
  </div>
)

const CompleteScreen: React.FC<{ onClose: () => void; votedCount: number }> = ({ onClose, votedCount }) => (
  <div className="flex flex-col items-center justify-center flex-1 text-center p-5">
    <h2 className="text-xl font-semibold text-foreground m-0 mb-2">
      <Trans id="swipemode.complete.title">All Done</Trans>
    </h2>
    <p className="text-sm text-muted m-0 mb-2">
      <Trans id="swipemode.complete.noposts">No more posts to vote on.</Trans>
    </p>
    {votedCount > 0 && (
      <p className="text-sm text-muted m-0 mb-2">
        <Trans id="swipemode.complete.votedcount">You voted on <strong className="text-primary">{votedCount}</strong> posts.</Trans>
      </p>
    )}
    <Button variant="primary" className="mt-4" onClick={onClose}>
      <Trans id="swipemode.complete.return">Return to Posts</Trans>
    </Button>
  </div>
)
