import React, { useState, useCallback, useEffect } from "react"
import { Post, Tag, Comment } from "@fider/models"
import { Markdown, Icon, Avatar, UserName, ImageGallery } from "@fider/components"
import { VoteSectionCompact } from "@fider/pages/ShowPost/components/VoteSectionCompact"
import { ResponseDetails } from "@fider/components/post/ShowPostResponse"
import { TagsPanel } from "@fider/pages/ShowPost/components/TagsPanel"
import { DiscussionPanel } from "@fider/pages/ShowPost/components/DiscussionPanel"
import { heroiconsExternalLink as IconOpen } from "@fider/icons.generated"
import { HStack, VStack } from "@fider/components/layout"
import { Fider, formatDate } from "@fider/services"
import { Moment } from "@fider/components"
import { Trans } from "@lingui/react/macro"
import { i18n } from "@lingui/core"
import "./SwipeCard.scss"

interface SwipeCardProps {
  post: Post
  tags: Tag[]
  attachments?: string[]
  comments?: Comment[]
  isActive: boolean
  controlMode: "swipe" | "buttons"
  onSwipe?: (direction: "left" | "right") => void
  onOpenPost?: () => void
  onSwipeProgress?: (progress: number, direction: "left" | "right" | null) => void
  style?: React.CSSProperties
}

const SWIPE_THRESHOLD = 80
const SWIPE_DEAD_ZONE = 25
const ROTATION_FACTOR = 0.05
const FLY_ANIMATION_MS = 250

export const SwipeCard: React.FC<SwipeCardProps> = ({
  post,
  tags,
  attachments,
  comments,
  isActive,
  controlMode,
  onSwipe,
  onOpenPost,
  onSwipeProgress,
  style,
}) => {
  const [isDragging, setIsDragging] = useState(false)
  const [startX, setStartX] = useState(0)
  const [startY, setStartY] = useState(0)
  const [offsetX, setOffsetX] = useState(0)
  const [isFlying, setIsFlying] = useState(false)
  const [flyDirection, setFlyDirection] = useState<"left" | "right" | null>(null)
  const [isScrolling, setIsScrolling] = useState(false)

  const handleStart = useCallback((clientX: number, clientY: number) => {
    if (!isActive || isFlying || controlMode === "buttons") return
    setIsDragging(true)
    setStartX(clientX)
    setStartY(clientY)
    setIsScrolling(false)
  }, [isActive, isFlying, controlMode])

  const handleMove = useCallback((clientX: number, clientY: number) => {
    if (!isDragging || isScrolling || controlMode === "buttons") return
    
    const deltaX = clientX - startX
    const deltaY = clientY - startY
    
    if (Math.abs(deltaY) > Math.abs(deltaX) && Math.abs(deltaX) < SWIPE_DEAD_ZONE) {
      setIsScrolling(true)
      setOffsetX(0)
      return
    }
    
    if (Math.abs(deltaX) > SWIPE_DEAD_ZONE) {
      const adjustedX = deltaX > 0 ? deltaX - SWIPE_DEAD_ZONE : deltaX + SWIPE_DEAD_ZONE
      setOffsetX(adjustedX)
    } else {
      setOffsetX(0)
    }
  }, [isDragging, startX, startY, isScrolling, controlMode])

  const handleEnd = useCallback(() => {
    if (!isDragging) return
    setIsDragging(false)
    
    if (isScrolling || controlMode === "buttons") {
      setOffsetX(0)
      return
    }
    
    if (Math.abs(offsetX) >= SWIPE_THRESHOLD) {
      const direction = offsetX > 0 ? "right" : "left"
      setFlyDirection(direction)
      setIsFlying(true)
      
      setTimeout(() => {
        onSwipe?.(direction)
        setIsFlying(false)
        setFlyDirection(null)
        setOffsetX(0)
      }, FLY_ANIMATION_MS)
    } else {
      setOffsetX(0)
    }
  }, [isDragging, offsetX, onSwipe, isScrolling, controlMode])

  const handleTouchStart = (e: React.TouchEvent) => {
    handleStart(e.touches[0].clientX, e.touches[0].clientY)
  }

  const handleTouchMove = (e: React.TouchEvent) => {
    handleMove(e.touches[0].clientX, e.touches[0].clientY)
  }

  const handleTouchEnd = () => {
    handleEnd()
  }

  const handleMouseDown = (e: React.MouseEvent) => {
    if (controlMode === "buttons") return
    handleStart(e.clientX, e.clientY)
  }

  useEffect(() => {
    if (!isDragging) return

    const handleMouseMove = (e: MouseEvent) => {
      handleMove(e.clientX, e.clientY)
    }

    const handleMouseUp = () => {
      handleEnd()
    }

    window.addEventListener("mousemove", handleMouseMove)
    window.addEventListener("mouseup", handleMouseUp)

    return () => {
      window.removeEventListener("mousemove", handleMouseMove)
      window.removeEventListener("mouseup", handleMouseUp)
    }
  }, [isDragging, handleMove, handleEnd])

  let translateX = controlMode === "buttons" ? 0 : offsetX
  if (isFlying) {
    translateX = flyDirection === "right" ? window.innerWidth + 200 : -window.innerWidth - 200
  }
  
  const rotation = controlMode === "buttons" ? 0 : offsetX * ROTATION_FACTOR
  const progress = Math.min(1, Math.abs(offsetX) / SWIPE_THRESHOLD)
  const swipeDirection = offsetX > 0 ? "right" : offsetX < 0 ? "left" : null

  useEffect(() => {
    if (onSwipeProgress && isActive) {
      onSwipeProgress(progress, swipeDirection)
    }
  }, [progress, swipeDirection, onSwipeProgress, isActive])

  const cardStyle: React.CSSProperties = {
    ...style,
    transform: isActive && controlMode === "swipe" ? `translateX(${translateX}px) rotate(${rotation}deg)` : undefined,
    transition: isDragging ? "none" : "transform 0.25s ease-out",
  }

  const postTags = tags.filter((tag) => post.tags.includes(tag.slug))

  return (
    <div
      className={`c-swipe-card ${isActive ? "c-swipe-card--active" : ""}`}
      style={cardStyle}
      onTouchStart={handleTouchStart}
      onTouchMove={handleTouchMove}
      onTouchEnd={handleTouchEnd}
      onMouseDown={handleMouseDown}
    >
      <div className="c-swipe-card__content">
        <div className="c-swipe-card__header">
          <HStack spacing={2} align="center">
            <Avatar user={post.user} />
            <VStack spacing={1}>
              <UserName user={post.user} />
              <span className="text-muted text-xs" data-tooltip={i18n._("showpost.createdat", { message: "Created {date}", date: formatDate(Fider.currentLocale, post.createdAt, "full") })}>
                <Trans id="showpost.lastactivity">Last activity:</Trans>{" "}
                <Moment locale={Fider.currentLocale} date={post.lastActivityAt} />
              </span>
            </VStack>
          </HStack>
          <button className="c-swipe-card__open-btn" onClick={(e) => { e.stopPropagation(); onOpenPost?.(); }}>
            <Icon sprite={IconOpen} className="h-4 w-4" />
          </button>
        </div>

        <h1 className="c-swipe-card__title">{post.title}</h1>

        {post.description && (
          <Markdown className="c-swipe-card__description" text={post.description} style="full" />
        )}
        {!post.description && (
          <em className="text-muted">
            <Trans id="showpost.message.nodescription">No description provided.</Trans>
          </em>
        )}

        {attachments && attachments.length > 0 && (
          <ImageGallery bkeys={attachments} />
        )}

        {postTags.length > 0 && (
          <div className="c-swipe-card__tags">
            <TagsPanel post={post} tags={tags} />
          </div>
        )}

        <VoteSectionCompact post={post} />

        <ResponseDetails status={post.status} response={post.response} />

        {comments && (
          <DiscussionPanel
            post={post}
            comments={comments}
            subscribed={false}
            reportedCommentIds={[]}
            dailyLimitReached={false}
          />
        )}
      </div>
    </div>
  )
}
