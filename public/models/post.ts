import { User } from "./identity"
import { DiscussionPermissions } from "./discussion"

export enum PostStatusValue {
  Open = "open",
  Planned = "planned",
  Started = "started",
  Completed = "completed",
  Declined = "declined",
  Duplicate = "duplicate",
  Deleted = "deleted",
  Archived = "archived",
}

export interface Post {
  sponsorPage?: string
  discussionPermissions: DiscussionPermissions
  permissions: PostPermissions
  id: number
  number: number
  slug: string
  title: string
  description: string
  createdAt: string
  lastActivityAt: string
  status: PostStatusValue
  user: User
  voteType: number
  voteRevision: number
  response: PostResponse | null
  votesCount: number
  commentsCount: number
  tags: string[]
  lockedSettings?: PostLockedSettings
  archivedSettings?: PostArchivedSettings
  upvotes?: number
  downvotes?: number
  moderationPending?: boolean
  moderationData?: string
}

export interface PostPermissions {
  edit: boolean
  delete: boolean
  respond: PostStatusValue[]
  lock: boolean
  archive: boolean
  moderate: boolean
  tag: boolean
  report: boolean
  viewVotes: boolean
  vote: boolean
  follow: boolean
}

export function isPostHidden(post: Post): boolean {
  return !!post.moderationPending
}

export function isPostLocked(post: Post): boolean {
  return !!post.lockedSettings && post.lockedSettings.locked;
}

export function isPostArchived(post: Post): boolean {
  return post.status === "archived";
}

export class PostStatus {
  constructor(public title: string, public value: PostStatusValue, public show: boolean, public closed: boolean, public filterable: boolean) {}

  public static Open = new PostStatus("Open", PostStatusValue.Open, false, false, true)
  public static Planned = new PostStatus("Planned", PostStatusValue.Planned, true, false, true)
  public static Started = new PostStatus("Started", PostStatusValue.Started, true, false, true)
  public static Completed = new PostStatus("Completed", PostStatusValue.Completed, true, true, true)
  public static Declined = new PostStatus("Declined", PostStatusValue.Declined, true, true, true)
  public static Duplicate = new PostStatus("Duplicate", PostStatusValue.Duplicate, true, true, true)
  public static Deleted = new PostStatus("Deleted", PostStatusValue.Deleted, false, true, false)
  public static Archived = new PostStatus("Archived", PostStatusValue.Archived, true, false, true)

  public static Get(value: string): PostStatus {
    if (value === PostStatus.Deleted.value) {
      return PostStatus.Deleted
    }
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

export interface Tag {
  permissions: { assign: boolean }
  id: number
  slug: string
  name: string
  color: string
  isPublic: boolean
}

export interface Vote {
  createdAt: Date
  user: Pick<User, "id" | "name" | "email" | "avatarURL" | "permissions">
}
