import { User } from "./identity"
import { Post, ReactionCount } from "./post"

export type PageStatus = "draft" | "published" | "unpublished" | "scheduled"
export type PageVisibility = "public" | "private" | "unlisted"
export type PinnedLocation = "footer" | "subheader"

export interface Page {
  id: number
  title: string
  slug: string
  content: string
  excerpt?: string
  bannerImageBKey?: string
  status: PageStatus
  visibility: PageVisibility
  allowedRoles?: string[]
  parentPageId?: number
  allowComments: boolean
  allowReactions: boolean
  showToc: boolean
  scheduledFor?: string
  publishedAt?: string
  createdAt: string
  updatedAt: string
  createdBy: User
  updatedBy: User
  authors?: User[]
  topics?: PageTopic[]
  tags?: PageTag[]
  metaDescription?: string
  canonicalUrl?: string
  commentsCount: number
  reactionCounts?: ReactionCount[]
  embeddedPosts?: Post[]
  cachedAt?: string
}

export interface PageTopic {
  id: number
  name: string
  slug: string
  description?: string
  color?: string
}

export interface PageTag {
  id: number
  name: string
  slug: string
}

export interface PageDraft {
  id: number
  pageId: number
  title: string
  slug: string
  content: string
  excerpt?: string
  bannerImageBKey?: string
  metaDescription?: string
  showToc: boolean
  updatedAt: string
}

