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
  return (
    <div 
      className={classSet({
        "p-3 cursor-pointer transition-colors border-b border-surface-alt hover:bg-surface-alt": true,
        "bg-accent-light": isSelected,
      })}
      onClick={() => onSelect(user)}
    >
      <HStack spacing={4}>
        <Avatar user={user} />
        <VStack spacing={0}>
          <span className="text-sm font-medium text-foreground">{user.name}</span>
          <span className="flex gap-1 flex-wrap mt-1">
            {user.role === UserRole.Administrator && (
              <span className="text-xs px-1 py-0.5 rounded bg-surface-alt text-muted">admin</span>
            )}
            {user.role === UserRole.Collaborator && (
              <span className="text-xs px-1 py-0.5 rounded bg-surface-alt text-muted">collab</span>
            )}
            {user.role === UserRole.Moderator && (
              <span className="text-xs px-1 py-0.5 rounded bg-surface-alt text-muted">mod</span>
            )}
            {user.role === UserRole.Helper && (
              <span className="text-xs px-1 py-0.5 rounded bg-surface-alt text-muted">helper</span>
            )}
            {user.status === UserStatus.Blocked && (
              <span className="text-xs px-1 py-0.5 rounded bg-danger-light text-danger">blocked</span>
            )}
          </span>
        </VStack>
      </HStack>
    </div>
  )
}
