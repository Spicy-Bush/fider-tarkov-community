import { ReactionCount } from "./post"
import { User } from "./identity"

export type DiscussionSort = "liked" | "disliked" | "replies" | "latest"

export interface DiscussionOwner {
  kind: "post" | "page"
  id: number
  number?: number
  title: string
  url: string
}

export interface DiscussionPermissions {
  comment: boolean
  react: boolean
  images: boolean
}

export interface DiscussionComment {
  id: number
  content: string
  createdAt: string
  user: User | null
  attachments?: string[]
  reactionCounts?: ReactionCount[]
  editedAt?: string
  editedBy?: User
  moderationPending?: boolean
  moderationData?: string
  parentId: number | null
  hasReplies: boolean
  state: "visible" | "hidden" | "deleted"
  permissions: {
    edit: boolean
    delete: boolean
    moderate: boolean
    reply: boolean
    react: boolean
    report: boolean
  }
}

export interface DiscussionPage {
  owner: DiscussionOwner
  permissions: DiscussionPermissions
  comments: DiscussionComment[]
  replies?: DiscussionComment[]
  next?: string
}

export interface CommentContext extends DiscussionPage {
  commentId: number
  nextAncestorId?: number
}
