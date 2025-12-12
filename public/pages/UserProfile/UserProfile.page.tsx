import React from "react"
import { UserRole, UserStatus, UserAvatarType, UserSettings, VisualRole } from "@fider/models"
import { UserProfile } from "@fider/components/UserProfile"

interface UserProfilePageProps {
  user: {
    id: number
    name: string
    role: UserRole
    visualRole?: VisualRole
    avatarURL: string
    status: UserStatus
    avatarType: UserAvatarType
  }
  userSettings?: UserSettings
}

export default function UserProfilePage({ user, userSettings }: UserProfilePageProps) {
  return (
    <div className="container">
      <UserProfile userId={user.id} user={{ ...user, visualRole: user.visualRole }}>
        <UserProfile.Header />
        <UserProfile.Actions />
        <UserProfile.Status />
        <UserProfile.Tabs>
          <UserProfile.Search />
          <UserProfile.Standing />
          <UserProfile.Settings userSettings={userSettings} />
        </UserProfile.Tabs>
      </UserProfile>
    </div>
  )
}
