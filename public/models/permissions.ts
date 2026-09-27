export interface SessionPermissions {
  readSettings: boolean
  manageProfanity: boolean
  createUsers: boolean
  viewDesignSystem: boolean
  manageAPIKeys: boolean
  changeOwnEmail: boolean
  enableEmailNotifications: boolean
  manageSettings: boolean
  manageContentSettings: boolean
  managePages: boolean
  managePageTopics: boolean
  manageNavigation: boolean
  manageTags: boolean
  manageMembers: boolean
  manageReports: boolean
  manageReportReasons: boolean
  manageQueue: boolean
  manageArchive: boolean
  manageResponses: boolean
  readResponses: boolean
  manageSponsorship: boolean
  manageWebhooks: boolean
  manageAuthentication: boolean
  manageInvitations: boolean
  manageBilling: boolean
  manageFiles: boolean
  exportBackup: boolean
  readProfiles: boolean
  editUserProfiles: boolean
  moderateUsers: boolean
  deleteUserModeration: boolean
  expireUserModeration: boolean
  blockUsers: boolean
  changeUserRoles: boolean
  changeUserVisualRoles: boolean
  readUserEmails: boolean
  createPosts: boolean
  editPosts: boolean
  deletePosts: boolean
  respondToPosts: boolean
  lockPosts: boolean
  moderatePosts: boolean
  tagPosts: boolean
  viewPostVotes: boolean
}

export interface UserPermissions {
  readProfile: boolean
  editName: boolean
  editAvatar: boolean
  block: boolean
  moderate: boolean
  deleteModeration: boolean
  expireModeration: boolean
  changeRole: boolean
  changeVisualRole: boolean
}
