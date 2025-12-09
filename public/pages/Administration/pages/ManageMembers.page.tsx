import React, { useState, useCallback } from "react"
import { Input, Avatar, Button, Icon } from "@fider/components"
import { User, UserRole, UserStatus } from "@fider/models"
import IconSearch from "@fider/assets/images/heroicons-search.svg"
import IconX from "@fider/assets/images/heroicons-x.svg"
import IconArrowLeft from "@fider/assets/images/heroicons-arrow-left.svg"
import { Fider, classSet } from "@fider/services"
import { HStack, VStack } from "@fider/components/layout"
import { PageConfig } from "@fider/components/layouts"
import { UserProfile } from "@fider/components/UserProfile"

import "./ManageMembers.page.scss"

export const pageConfig: PageConfig = {
  title: "Members",
  subtitle: "Manage your site administrators and collaborators",
  sidebarItem: "members",
  layoutVariant: "fullWidth",
}

interface ManageMembersPageProps {
  users: User[]
}

interface UserListItemProps {
  user: User
  isSelected: boolean
  onSelect: (user: User) => void
}

const UserListItem: React.FC<UserListItemProps> = ({ user, isSelected, onSelect }) => {
  const admin = user.role === UserRole.Administrator && <span className="c-member-item__role">admin</span>
  const collaborator = user.role === UserRole.Collaborator && <span className="c-member-item__role">collab</span>
  const moderator = user.role === UserRole.Moderator && <span className="c-member-item__role">mod</span>
  const helper = user.role === UserRole.Helper && <span className="c-member-item__role">helper</span>
  const blocked = user.status === UserStatus.Blocked && <span className="c-member-item__role c-member-item__role--blocked">blocked</span>
  
  const canViewEmails = Fider.session.user.isAdministrator || Fider.session.user.isCollaborator

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
          {canViewEmails && user.email && (
            <span className="c-member-item__email">{user.email}</span>
          )}
          <span className="c-member-item__roles">
            {admin} {moderator} {helper} {collaborator} {blocked}
          </span>
        </VStack>
      </HStack>
    </div>
  )
}

const sortByStaff = (left: User, right: User) => {
  const rolePriority: { [key in UserRole]: number } = {
    [UserRole.Administrator]: 1,
    [UserRole.Collaborator]: 2,
    [UserRole.Moderator]: 3,
    [UserRole.Helper]: 4,
    [UserRole.Visitor]: 5,
  }

  if (rolePriority[left.role] === rolePriority[right.role]) {
    return left.name.localeCompare(right.name)
  }
  return rolePriority[left.role] - rolePriority[right.role]
}

const ManageMembersPage: React.FC<ManageMembersPageProps> = (props) => {
  const [query, setQuery] = useState("")
  const [allUsers, setAllUsers] = useState<User[]>(() => [...props.users].sort(sortByStaff))
  const [filteredUsers, setFilteredUsers] = useState<User[]>(() => [...props.users].sort(sortByStaff))
  const [visibleUsers, setVisibleUsers] = useState<User[]>(() => [...props.users].sort(sortByStaff).slice(0, 20))
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [profileKey, setProfileKey] = useState(0)

  const canViewEmails = Fider.session.user.isAdministrator || Fider.session.user.isCollaborator

  const memberFilter = useCallback((searchQuery: string, user: User): boolean => {
    const queryLower = searchQuery.toLowerCase();
    
    if (user.name.toLowerCase().indexOf(queryLower) >= 0) {
      return true;
    }
    
    if (canViewEmails && user.email && user.email.toLowerCase().indexOf(queryLower) >= 0) {
      return true;
    }

    if (user.providers && user.providers.length > 0) {
      for (const provider of user.providers) {
        if (provider.uid.toLowerCase().indexOf(queryLower) >= 0) {
          return true;
        }
      }
    }
    
    return false;
  }, [canViewEmails])

  const handleSearchFilterChanged = useCallback((newQuery: string) => {
    const users = allUsers
      .filter((x) => memberFilter(newQuery, x))
      .sort(sortByStaff)
    setQuery(newQuery)
    setFilteredUsers(users)
    setVisibleUsers(users.slice(0, 20))
  }, [allUsers, memberFilter])

  const showMore = () => {
    setVisibleUsers(filteredUsers.slice(0, visibleUsers.length + 20))
  }

  const clearSearch = () => {
    handleSearchFilterChanged("")
  }

  const handleSelectUser = (user: User) => {
    setSelectedUser(user)
    setProfileKey(prev => prev + 1)
  }

  const updateUserInLists = useCallback((updatedUser: Partial<User> & { id: number }) => {
    setAllUsers(prev => prev.map(u => u.id === updatedUser.id ? { ...u, ...updatedUser } : u))
    setFilteredUsers(prev => prev.map(u => u.id === updatedUser.id ? { ...u, ...updatedUser } : u))
    setVisibleUsers(prev => prev.map(u => u.id === updatedUser.id ? { ...u, ...updatedUser } : u))
    setSelectedUser(prev => prev && prev.id === updatedUser.id ? { ...prev, ...updatedUser } : prev)
  }, [])

  const searchPlaceholder = canViewEmails 
    ? "Search by name / email / provider..."
    : "Search by name / provider...";

  return (
    <div className="c-members-split-view">
      <div className="c-members-split-view__list">
        <div className="c-members-split-view__search">
          <Input
            field="query"
            icon={query ? IconX : IconSearch}
            onIconClick={query ? clearSearch : undefined}
            placeholder={searchPlaceholder}
            value={query}
            onChange={handleSearchFilterChanged}
          />
        </div>
        <div className="c-members-split-view__list-content">
          {visibleUsers.map((user) => (
            <UserListItem 
              key={user.id} 
              user={user} 
              isSelected={selectedUser?.id === user.id}
              onSelect={handleSelectUser}
            />
          ))}
          {visibleUsers.length < filteredUsers.length && (
            <div className="c-members-split-view__load-more">
              <Button variant="tertiary" onClick={showMore}>
                Load more ({filteredUsers.length - visibleUsers.length} remaining)
              </Button>
            </div>
          )}
        </div>
        <div className="c-members-split-view__footer">
          {filteredUsers.length} users {query && `matching "${query}"`}
        </div>
      </div>

      <div className={classSet({
        "c-members-split-view__preview": true,
        "c-members-split-view__preview--mobile-open": selectedUser !== null,
      })}>
        {selectedUser ? (
          <>
            <Button 
              variant="tertiary" 
              size="small" 
              className="c-members-split-view__back-btn"
              onClick={() => setSelectedUser(null)}
            >
              <Icon sprite={IconArrowLeft} className="h-4" />
              <span>Back to list</span>
            </Button>
            <UserProfile 
              key={profileKey}
              userId={selectedUser.id} 
              user={{
                id: selectedUser.id,
                name: selectedUser.name,
                avatarURL: selectedUser.avatarURL,
                role: selectedUser.role,
                status: selectedUser.status,
              }} 
              embedded 
              compact
              onUserUpdate={(updates) => {
                const safeUpdates: Partial<User> & { id: number } = { id: selectedUser.id }
                if (updates.name !== undefined) safeUpdates.name = updates.name
                if (updates.avatarURL !== undefined) safeUpdates.avatarURL = updates.avatarURL
                updateUserInLists(safeUpdates)
              }}
            >
              <UserProfile.Header compact />
              <UserProfile.Details 
                email={selectedUser.email}
                providers={selectedUser.providers}
                onRoleChange={(role) => updateUserInLists({ id: selectedUser.id, role })}
                onVisualRoleChange={(visualRole) => updateUserInLists({ id: selectedUser.id, visualRole })}
              />
              <UserProfile.Actions />
              <UserProfile.Status />
              <UserProfile.Tabs>
                <UserProfile.Search />
                <UserProfile.Standing />
              </UserProfile.Tabs>
            </UserProfile>
          </>
        ) : (
          <div className="c-members-split-view__empty">
            <p className="text-muted">Select a member to view their profile</p>
          </div>
        )}
      </div>
    </div>
  )
}

export default ManageMembersPage
