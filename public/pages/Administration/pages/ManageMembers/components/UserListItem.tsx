import React from "react"
import { Avatar } from "@fider/components"
import { classSet } from "@fider/services"
import { HStack, VStack } from "@fider/components/layout"
import { User, UserRole, UserStatus } from "@fider/models"

export interface UserListItemProps {
  user: User
  isSelected: boolean
  onSelect: (user: User) => void
}

export const UserListItem: React.FC<UserListItemProps> = ({
  user,
  isSelected,
  onSelect,
}) => {
  const admin = user.role === UserRole.Administrator && (
    <span className="c-member-item__role">admin</span>
  )
  const collaborator = user.role === UserRole.Collaborator && (
    <span className="c-member-item__role">collab</span>
  )
  const moderator = user.role === UserRole.Moderator && (
    <span className="c-member-item__role">mod</span>
  )
  const helper = user.role === UserRole.Helper && (
    <span className="c-member-item__role">helper</span>
  )
  const blocked = user.status === UserStatus.Blocked && (
    <span className="c-member-item__role c-member-item__role--blocked">blocked</span>
  )

  const className = classSet({
    "c-member-item": true,
    "c-member-item--selected": isSelected,
  })

  return (
    <div className={className} onClick={() => onSelect(user)}>
      <HStack spacing={4}>
        <Avatar user={user} />
        <VStack spacing={0}>
          <span className="c-member-item__name">{user.name}</span>
          <span className="c-member-item__roles">
            {admin} {moderator} {helper} {collaborator} {blocked}
          </span>
        </VStack>
      </HStack>
    </div>
  )
}

