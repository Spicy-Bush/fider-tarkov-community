import React, { useRef, useMemo, useState, useEffect } from "react"
import { Page, Post } from "@fider/models"
import { Markdown, Button } from "@fider/components"
import { VStack, HStack } from "@fider/components/layout"
import { togglePageReaction } from "@fider/services/pages"
import { useTableOfContents } from "@fider/hooks"
import { notify } from "@fider/services"
import { EmbeddedPostsList } from "@fider/components/page/EmbeddedPostsList"
import { Discussion } from "@fider/components/discussion/Discussion"
import { Reactions } from "@fider/components/post/Reactions"
import { AdSlot, useAdSelection } from "@fider/components/sponsorship"

interface ViewPageProps {
  page: Page
}

type ContentPart = 
  | { type: "text"; content: string }
  | { type: "posts"; postIds: number[] | null; filters: Record<string, string> | null }

const ViewPage = ({ page }: ViewPageProps) => {
  const contentRef = useRef<HTMLDivElement>(null)
  const emojiSelectorRef = useRef<HTMLDivElement>(null)
  const { toc, activeId, scrollTo } = useTableOfContents(contentRef, page.title, "page-title")
  const [reactionCounts, setReactionCounts] = useState(page.reactionCounts || [])
  const { ads: pageAds, loaded: pageAdLoaded, error: pageAdError } = useAdSelection([
    { instanceId: "pages-header", placementId: "pages_header" },
  ])

  const readingTime = useMemo(() => Math.max(1, Math.ceil(page.content.split(/\s+/).length / 250)), [page.content])

  const contentParts = useMemo(() => {
    const parts: ContentPart[] = []
    
    const tableRegex = /<table\s+type=posts\s+filters=(?:"([^"]+)"|(\S+))(?:\s+limit=(\d+))?\s*\/>/g
    const postRegex = /<post\s+id=(\d+)\s*\/>/g
    const postsRegex = /<posts\s+ids=(?:"([^"]+)"|(\S+))(?:\s+limit=(\d+))?\s*\/>/g
    
    interface EmbedMatch {
      index: number
      length: number
      postIds: number[] | null
      filters: Record<string, string> | null
    }
    
    const allMatches: EmbedMatch[] = []
    
    let match
    while ((match = tableRegex.exec(page.content)) !== null) {
      const filterStr = match[1] || match[2] || ""
      const filters: Record<string, string> = {}
      filterStr.split(/\s+/).forEach(part => {
        if (part.includes(":")) {
          const [key, val] = part.split(":", 2)
          filters[key] = val
        } else if (part) {
          filters["tag"] = part
        }
      })
      allMatches.push({ index: match.index, length: match[0].length, postIds: null, filters })
    }
    
    while ((match = postRegex.exec(page.content)) !== null) {
      const id = parseInt(match[1], 10)
      allMatches.push({ index: match.index, length: match[0].length, postIds: [id], filters: null })
    }
    
    while ((match = postsRegex.exec(page.content)) !== null) {
      const idsStr = match[1] || match[2] || ""
      const ids = idsStr.split(",").map(s => parseInt(s.trim(), 10)).filter(n => !isNaN(n))
      allMatches.push({ index: match.index, length: match[0].length, postIds: ids, filters: null })
    }
    
    allMatches.sort((a, b) => a.index - b.index)
    
    let lastIndex = 0
    for (const embedMatch of allMatches) {
      if (embedMatch.index > lastIndex) {
        parts.push({ type: "text", content: page.content.substring(lastIndex, embedMatch.index) })
      }
      parts.push({ type: "posts", postIds: embedMatch.postIds, filters: embedMatch.filters })
      lastIndex = embedMatch.index + embedMatch.length
    }
    
    if (lastIndex < page.content.length) {
      parts.push({ type: "text", content: page.content.substring(lastIndex) })
    }
    
    if (parts.length === 0) {
      parts.push({ type: "text", content: page.content })
    }
    
    return parts
  }, [page.content])

  const getPostsForPart = (part: ContentPart): Post[] => {
    if (part.type !== "posts" || !page.embeddedPosts) return []
    
    if (part.postIds && part.postIds.length > 0) {
      const postsById = new Map(page.embeddedPosts.map(p => [p.id, p]))
      return part.postIds.map(id => postsById.get(id)).filter((p): p is Post => p !== undefined)
    }
    
    if (part.filters) {
      return page.embeddedPosts.filter(post => {
        if (part.filters!.tag && !post.tags.includes(part.filters!.tag)) return false
        if (part.filters!.status && post.status !== part.filters!.status) return false
        return true
      })
    }
    
    return page.embeddedPosts
  }

  useEffect(() => {
    const images = contentRef.current?.querySelectorAll('img')
    images?.forEach(img => {
      if (img.src.startsWith(window.location.origin)) {
        img.classList.add('max-w-full', 'rounded-card', 'my-4')
      }
    })
  }, [contentParts])

  const toggleReaction = async (emoji: string) => {
    const result = await togglePageReaction(page.id, emoji)
    if (result.ok) {
      const existingReaction = reactionCounts.find((r) => r.emoji === emoji)
      if (existingReaction) {
        setReactionCounts(
          reactionCounts.map((r) =>
            r.emoji === emoji
              ? { ...r, count: r.includesMe ? r.count - 1 : r.count + 1, includesMe: !r.includesMe }
              : r
          ).filter((r) => r.count > 0)
        )
      } else {
        setReactionCounts([...reactionCounts, { emoji, count: 1, includesMe: true }])
      }
    } else {
      notify.error("Failed to toggle reaction")
    }
  }

  const shareOnTwitter = () => {
    const url = encodeURIComponent(window.location.href)
    const text = encodeURIComponent(page.title)
    window.open(`https://twitter.com/intent/tweet?url=${url}&text=${text}`, "_blank")
  }

  return (
    <div className="page container">
      {page.showToc && toc.length > 0 && (
        <nav className="lg:hidden mb-6 bg-elevated border border-border rounded-panel p-4">
          <h3 className="text-subtitle mb-3">Table of Contents</h3>
          <VStack spacing={1}>
            {toc.map((item) => (
              <div
                key={item.id}
                className={`cursor-pointer text-sm flex gap-2 ${
                  item.level === 2 ? "ml-4" : item.level === 3 ? "ml-8" : ""
                } ${activeId === item.id ? "text-primary font-semibold" : "text-muted hover:text-foreground"}`}
                onClick={() => scrollTo(item.id)}
              >
                {item.number && <span className="text-muted shrink-0">{item.number}</span>}
                <span>{item.text}</span>
              </div>
            ))}
          </VStack>
        </nav>
      )}
      <div className={page.showToc && toc.length > 0 ? "lg:grid lg:gap-6 lg:grid-cols-[1fr_4fr_1fr]" : "max-w-4xl mx-auto"}>
        {page.showToc && toc.length > 0 && (
          <aside className="hidden lg:block lg:col-start-1 lg:col-end-2">
            <div className="sticky top-8 bg-elevated border border-border rounded-panel p-4">
              <h3 className="text-subtitle mb-4">Table of Contents</h3>
              <VStack spacing={2}>
                {toc.map((item) => (
                  <div
                    key={item.id}
                    className={`cursor-pointer text-sm flex gap-2 ${
                      item.level === 2 ? "ml-3" : item.level === 3 ? "ml-6" : ""
                    } ${activeId === item.id ? "text-primary font-semibold" : "text-muted hover:text-foreground"}`}
                    onClick={() => scrollTo(item.id)}
                  >
                    {item.number && <span className="text-muted shrink-0">{item.number}</span>}
                    <span>{item.text}</span>
                  </div>
                ))}
              </VStack>
            </div>
          </aside>
        )}
        <div className={`min-w-0 overflow-hidden ${page.showToc && toc.length > 0 ? "lg:col-start-2 lg:col-end-3" : ""}`}>
          <div className="relative mb-4">
            {page.bannerImageBKey && (
              <img 
                src={`/static/images/${page.bannerImageBKey}`} 
                alt={page.title} 
                className="w-full h-16 object-cover"
                style={{ maskImage: "linear-gradient(to bottom, black 0%, transparent 100%)", WebkitMaskImage: "linear-gradient(to bottom, black 0%, transparent 100%)" }}
                fetchPriority="high"
                loading="eager"
                decoding="async"
              />
            )}
            <h1 
              id="page-title"
              className="text-large"
              style={{ marginTop: page.bannerImageBKey ? "-0.8rem" : "0" }}
            >
              {page.title}
            </h1>
          </div>

          <HStack spacing={4} className="text-sm text-muted mb-6">
            <span>Updated {new Date(page.updatedAt).toLocaleDateString()}</span>
            <span>{readingTime} min read</span>
            {page.authors && page.authors.length > 0 && <span>By {page.authors.map((author) => author.name).join(", ")}</span>}
          </HStack>

          <AdSlot
            instanceId="pages-header"
            placementId="pages_header"
            ad={pageAdLoaded && !pageAdError ? (pageAds["pages-header"] ?? null) : undefined}
            selectFailed={pageAdError}
            className="mb-6"
          />

          <div ref={contentRef} className="c-markdown mb-8 min-w-0 max-w-full">
            {contentParts.map((part, index) => {
              if (part.type === "text") {
                return <Markdown key={index} text={part.content} style="full" embedImages={true} />
              }
              const postsToShow = getPostsForPart(part)
              if (postsToShow.length === 0) return null
              return (
                <div key={index} className="my-6 p-4 bg-tertiary rounded-card border border-border min-w-0 max-w-full">
                  <EmbeddedPostsList posts={postsToShow} />
                </div>
              )
            })}
          </div>

          {page.allowReactions && (
            <div className="mb-8">
              <Reactions reactions={reactionCounts} emojiSelectorRef={emojiSelectorRef} toggleReaction={toggleReaction} />
            </div>
          )}

          <HStack spacing={4} className="mb-8">
            <Button variant="primary" onClick={shareOnTwitter}>
              X
            </Button>
          </HStack>

          <div className="mt-8">
            <Discussion owner={{ kind: "page", id: page.id, title: page.title, url: `/pages/${page.slug}` }} />
          </div>
        </div>

      </div>
    </div>
  )
}

export default ViewPage
