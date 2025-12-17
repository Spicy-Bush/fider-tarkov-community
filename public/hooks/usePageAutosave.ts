import { useState, useEffect, useRef } from "react"
import { http } from "@fider/services"

interface DraftData {
  title: string
  slug: string
  content: string
  excerpt: string
  showToc: boolean
}

export const usePageAutosave = (pageId: number, data: DraftData) => {
  const [lastSaved, setLastSaved] = useState(new Date())
  const [hasChanges, setHasChanges] = useState(false)
  const timeoutRef = useRef<NodeJS.Timeout>()

  useEffect(() => {
    if (!pageId) return

    setHasChanges(true)

    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current)
    }

    timeoutRef.current = setTimeout(async () => {
      try {
        await http.post(`/_api/pages/${pageId}/draft`, data)
        setLastSaved(new Date())
        setHasChanges(false)
      } catch (error) {
        console.error("Failed to autosave:", error)
      }
    }, 5000)

    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current)
      }
    }
  }, [pageId, data.title, data.content, data.excerpt, data.showToc])

  return { lastSaved, hasChanges }
}
