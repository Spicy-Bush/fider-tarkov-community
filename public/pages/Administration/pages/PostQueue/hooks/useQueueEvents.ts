import { useEffect } from "react"
import { Post } from "@fider/models"
import { actions, queueEventSource } from "@fider/services"
import { QueuePostNewEvent, QueuePostTaggedEvent } from "@fider/models"

interface UseQueueEventsConfig {
  postsRef: React.MutableRefObject<Post[]>
  selectedPostRef: React.MutableRefObject<Post | null>
  currentUserIdRef: React.MutableRefObject<number>
  setPosts: React.Dispatch<React.SetStateAction<Post[]>>
  setTotal: React.Dispatch<React.SetStateAction<number>>
  setNewPostIds: React.Dispatch<React.SetStateAction<Set<number>>>
  setTaggedByOtherIds: React.Dispatch<React.SetStateAction<Set<number>>>
}

export const useQueueEvents = (config: UseQueueEventsConfig): void => {
  const {
    postsRef,
    selectedPostRef,
    currentUserIdRef,
    setPosts,
    setTotal,
    setNewPostIds,
    setTaggedByOtherIds,
  } = config

  useEffect(() => {
    queueEventSource.connect()

    const unsubNewPost = queueEventSource.on("queue.post_new", async (_, payload) => {
      const data = payload as QueuePostNewEvent
      const isUntaggedByMe = data.untaggedByUserId === currentUserIdRef.current

      if (isUntaggedByMe && data.postNumber) {
        const result = await actions.getPost(data.postNumber)
        if (result.ok && result.data) {
          setPosts((prev) => {
            if (prev.some((p) => p.id === data.postId)) {
              return prev
            }
            return [result.data!, ...prev]
          })
          setTotal((prev) => prev + 1)
        }
      } else {
        const alreadyInList = postsRef.current.some((p) => p.id === data.postId)
        if (alreadyInList) {
          setTaggedByOtherIds((prev) => {
            const newSet = new Set(prev)
            newSet.delete(data.postId)
            return newSet
          })
        } else {
          setNewPostIds((prev) => new Set(prev).add(data.postId))
        }
      }
    })

    const unsubTagged = queueEventSource.on("queue.post_tagged", (_, payload) => {
      const data = payload as QueuePostTaggedEvent
      const isTaggedByMe = data.taggedByUserId === currentUserIdRef.current
      if (isTaggedByMe) {
        setPosts((prev) => prev.filter((p) => p.id !== data.postId))
        setTotal((prev) => Math.max(0, prev - 1))
      } else if (selectedPostRef.current?.id === data.postId) {
        setTaggedByOtherIds((prev) => new Set(prev).add(data.postId))
      } else {
        setPosts((prev) => prev.filter((p) => p.id !== data.postId))
        setTotal((prev) => Math.max(0, prev - 1))
      }
    })

    return () => {
      unsubNewPost()
      unsubTagged()
      queueEventSource.disconnect()
    }
  }, [])
}

