import { User } from "./identity"

export interface Post {
  id: number
  number: number
  slug: string
  title: string
  description: string
  createdAt: string
  lastActivityAt: string
  status: string
  user: User
  voteType: number
  response: PostResponse | null
  votesCount: number
  commentsCount: number
  tags: string[]
  tagDates?: string
  lockedSettings?: PostLockedSettings
  archivedSettings?: PostArchivedSettings
  upvotes?: number
  downvotes?: number
}

export function isPostLocked(post: Post): boolean {
  return !!post.lockedSettings && post.lockedSettings.locked;
}

export function isPostArchived(post: Post): boolean {
  return post.status === "archived";
}

export class PostStatus {
  constructor(public title: string, public value: string, public show: boolean, public closed: boolean, public filterable: boolean) {}

  public static Open = new PostStatus("Open", "open", false, false, true)
  public static Planned = new PostStatus("Planned", "planned", true, false, true)
  public static Started = new PostStatus("Started", "started", true, false, true)
  public static Completed = new PostStatus("Completed", "completed", true, true, true)
  public static Declined = new PostStatus("Declined", "declined", true, true, true)
  public static Duplicate = new PostStatus("Duplicate", "duplicate", true, true, true)
  public static Deleted = new PostStatus("Deleted", "deleted", false, true, true)
  public static Archived = new PostStatus("Archived", "archived", true, false, true)

  public static Get(value: string): PostStatus {
    for (const status of PostStatus.All) {
      if (status.value === value) {
        return status
      }
    }
    throw new Error(`PostStatus not found for value ${value}.`)
  }

  public static All = [PostStatus.Open, PostStatus.Planned, PostStatus.Started, PostStatus.Completed, PostStatus.Duplicate, PostStatus.Declined, PostStatus.Archived]
}

export interface PostLockedSettings {
  locked: boolean;
  lockedAt: string;
  lockedBy: User;
  lockMessage?: string;
}

export interface PostArchivedSettings {
  archivedAt: string;
  archivedBy: User;
  previousStatus: string;
}

export interface PostResponse {
  user: User
  text: string
  respondedAt: Date
  original?: {
    number: number
    title: string
    slug: string
    status: string
  }
}

export interface ReactionCount {
  emoji: string
  count: number
  includesMe: boolean
}

export interface Comment {
  id: number
  content: string
  createdAt: string
  user: User
  attachments?: string[]
  reactionCounts?: ReactionCount[]
  editedAt?: string
  editedBy?: User
}

export interface Tag {
  id: number
  slug: string
  name: string
  color: string
  isPublic: boolean
}

export interface Vote {
  createdAt: Date
  user: {
    id: number
    name: string
    email: string
    avatarURL: string
  }
}
