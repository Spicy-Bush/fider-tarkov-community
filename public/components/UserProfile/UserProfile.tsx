// UserProfile converted to Tailwind

import React, { ReactNode } from "react"
import { UserProfileProvider, UserData } from "./context"
import { UserProfileHeader } from "./UserProfileHeader"
import { UserProfileActions } from "./UserProfileActions"
import { UserProfileStatus } from "./UserProfileStatus"
import { UserProfileTabs } from "./UserProfileTabs"
import { UserProfileSearch } from "./UserProfileSearch"
import { UserProfileStanding } from "./UserProfileStanding"
import { UserProfileSettings } from "./UserProfileSettings"
import { UserProfileDetails } from "./UserProfileDetails"
import { classSet } from "@fider/services"

interface UserProfileProps {
  userId: number
  user?: UserData
  embedded?: boolean
  compact?: boolean
  onUserUpdate?: (user: Partial<UserData>) => void
  children: ReactNode
}

interface UserProfileComponent extends React.FC<UserProfileProps> {
  Header: typeof UserProfileHeader
  Actions: typeof UserProfileActions
  Status: typeof UserProfileStatus
  Tabs: typeof UserProfileTabs
  Search: typeof UserProfileSearch
  Standing: typeof UserProfileStanding
  Settings: typeof UserProfileSettings
  Details: typeof UserProfileDetails
}

const UserProfileRoot: React.FC<UserProfileProps> = ({
  userId,
  user,
  embedded = false,
  compact = false,
  onUserUpdate,
  children,
}) => {
  return (
    <UserProfileProvider userId={userId} user={user} embedded={embedded} compact={compact} onUserUpdate={onUserUpdate}>
      <div className={classSet({
        "max-w-[1200px] mx-auto p-5 pb-[calc(20px+100px)] grid grid-cols-1 gap-3 relative max-md:p-3 max-md:pb-[calc(12px+100px)]": true,
        "max-w-none p-0 m-0 bg-tertiary": embedded,
      })}>
        {children}
      </div>
    </UserProfileProvider>
  )
}

export const UserProfile = UserProfileRoot as UserProfileComponent
UserProfile.Header = UserProfileHeader
UserProfile.Actions = UserProfileActions
UserProfile.Status = UserProfileStatus
UserProfile.Tabs = UserProfileTabs
UserProfile.Search = UserProfileSearch
UserProfile.Standing = UserProfileStanding
UserProfile.Settings = UserProfileSettings
UserProfile.Details = UserProfileDetails
