import { Permission, PostStatus, PostStatusValue, UserRole, userRoleLabels } from "@fider/models"

type PermissionGroupID = "responses" | "posts" | "moderation" | "members" | "site" | "system"

export interface PermissionInfo {
  group: PermissionGroupID
  label: string
  description: string
  sensitive?: boolean
}

// Every editable permission needs a label before it can enter the matrix.
export const PERMISSIONS: Record<Permission, PermissionInfo> = {
  createPosts: { group: "posts", label: "Create posts", description: "Also limited by posting settings" },
  editPosts: { group: "posts", label: "Edit posts", description: "Own posts for an hour; staff can edit others' posts" },
  deletePosts: { group: "posts", label: "Delete posts", description: "Posts by members they can manage" },
  lockPosts: { group: "posts", label: "Lock posts", description: "Lock and unlock discussion" },
  tagPosts: { group: "posts", label: "Tag posts", description: "Apply tags within the tagging window" },
  viewPostVotes: { group: "posts", label: "View voters", description: "See who voted on a post" },
  moderatePosts: { group: "posts", label: "Review flagged content", description: "Approve or hide posts and comments held by moderation checks" },
  bypassContentRestrictions: { group: "posts", label: "Bypass content restrictions", description: "Post through site pauses and locked discussions" },
  bypassPostingRateLimits: { group: "posts", label: "Bypass posting rate limits", description: "Ignore configured post and comment rate limits" },
  tagPostsOutsideWindow: { group: "posts", label: "Tag outside the editing window", description: "Tag posts after the tagging window ends" },

  manageQueue: { group: "moderation", label: "Post queue", description: "Triage new posts" },
  manageReports: { group: "moderation", label: "Reports", description: "Review and resolve reports" },
  manageReportReasons: { group: "moderation", label: "Report reasons", description: "Edit the reasons members can pick" },
  manageArchive: { group: "moderation", label: "Archive", description: "Archive and restore posts" },
  exportFeedback: { group: "moderation", label: "Feedback exports", description: "Download posts and build sheets for the developers" },

  readProfiles: { group: "members", label: "View profiles", description: "Member profiles and standing" },
  manageMembers: { group: "members", label: "Member list", description: "Browse and search members" },
  editUserProfiles: { group: "members", label: "Edit profiles", description: "Change member names and avatars" },
  readUserEmails: { group: "members", label: "View emails", description: "See member email addresses", sensitive: true },
  moderateUsers: { group: "members", label: "Warn and mute", description: "Issue warnings and mutes" },
  expireUserModeration: { group: "members", label: "Expire warnings and mutes", description: "End them early" },
  deleteUserModeration: { group: "members", label: "Delete warnings and mutes", description: "Remove them from the record" },
  blockUsers: { group: "members", label: "Block members", description: "Block and unblock accounts" },
  changeUserVisualRoles: { group: "members", label: "Change visual roles", description: "Badges and name colours" },
  changeUserRoles: { group: "members", label: "Change roles", description: "Promote and demote members", sensitive: true },
  createUsers: { group: "members", label: "Create accounts", description: "Through the API", sensitive: true },

  readSettings: { group: "site", label: "View settings", description: "Open the admin area" },
  manageSettings: { group: "site", label: "Site settings", description: "General, advanced and privacy settings" },
  manageContentSettings: { group: "site", label: "Content settings", description: "Posting, reporting and content limits" },
  managePages: { group: "site", label: "Pages", description: "Create and edit pages" },
  managePageTopics: { group: "site", label: "Page topics", description: "Topics and tags for pages" },
  manageNavigation: { group: "site", label: "Navigation", description: "Header and footer links" },
  viewPrivateTags: { group: "site", label: "View private tags", description: "See tags hidden from other members" },
  manageTags: { group: "site", label: "Tags", description: "Create and edit post tags" },
  readResponses: { group: "site", label: "Use canned responses", description: "Insert saved responses" },
  manageResponses: { group: "site", label: "Canned responses", description: "Create and edit saved responses" },
  manageProfanity: { group: "site", label: "Profanity filter", description: "Edit blocked words" },
  manageSponsorship: { group: "site", label: "Sponsorship", description: "Ads, packages and campaigns" },
  manageWebhooks: { group: "site", label: "Webhooks", description: "Outgoing event hooks" },

  manageRolePermissions: { group: "system", label: "Role permissions", description: "Choose which actions each role can use", sensitive: true },
  manageAuthentication: { group: "system", label: "Authentication", description: "Manage sign-in methods", sensitive: true },
  manageInvitations: { group: "system", label: "Invitations", description: "Invite people by email" },
  manageBilling: { group: "system", label: "Billing", description: "Plans and payment details", sensitive: true },
  manageFiles: { group: "system", label: "Files", description: "Browse, rename and delete uploads" },
  exportBackup: { group: "system", label: "Full backups", description: "All data, including account credentials", sensitive: true },
  manageAPIKeys: { group: "system", label: "API keys", description: "Use and regenerate their API key", sensitive: true },
  viewDesignSystem: { group: "system", label: "Design system", description: "Component reference page" },
  changeOwnEmail: { group: "system", label: "Change own email", description: "Update their sign-in address" },
  enableEmailNotifications: { group: "system", label: "Email notifications", description: "Turn on email delivery for themselves" },
}

export interface PermissionGroup {
  id: string
  label: string
  permissions: Permission[]
}

const GROUPS: { id: PermissionGroupID; label: string }[] = [
  { id: "posts", label: "Posts" },
  { id: "responses", label: "Post responses" },
  { id: "moderation", label: "Moderation" },
  { id: "members", label: "Members" },
  { id: "site", label: "Site" },
  { id: "system", label: "System" },
]

export const PERMISSION_GROUPS: PermissionGroup[] = GROUPS.map((group) => ({
  ...group,
  permissions: (Object.keys(PERMISSIONS) as Permission[]).filter((permission) => PERMISSIONS[permission].group === group.id),
}))

export const ALL_PERMISSIONS: Permission[] = PERMISSION_GROUPS.flatMap((group) => group.permissions)

export interface RoleColumn {
  role: UserRole
  label: string
  className: string
}

export const ROLE_COLUMNS: RoleColumn[] = Object.entries(userRoleLabels).map(([role, label]) => ({
  role: role as UserRole,
  label,
  className: role === UserRole.Visitor ? "text-foreground" : `role-${label}`,
}))

export type PermissionCell = Permission | `response:${PostStatusValue}`

export function responseCell(status: PostStatusValue): PermissionCell {
  return `response:${status}`
}

export function responseStatus(cell: PermissionCell): PostStatusValue | undefined {
  return cell.startsWith("response:") ? cell.slice("response:".length) as PostStatusValue : undefined
}

export function permissionInfo(cell: PermissionCell): PermissionInfo {
  const status = responseStatus(cell)
  if (status !== undefined) {
    return { group: "responses", label: `Respond: ${PostStatus.Get(status).title}`, description: "Set this status and publish an official response" }
  }
  return PERMISSIONS[cell as Permission]
}

export function permissionGroups(cells: PermissionCell[]) {
  return GROUPS.map((group) => ({ ...group, permissions: cells.filter((cell) => permissionInfo(cell).group === group.id) }))
}
