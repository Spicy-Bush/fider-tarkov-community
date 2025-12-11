import React, { useState, useCallback } from "react"
import { Input, Button, Icon } from "@fider/components"
import { User, UserRole } from "@fider/models"
import { heroiconsSearch as IconSearch, heroiconsX as IconX, heroiconsArrowLeft as IconArrowLeft } from "@fider/icons.generated"
import { Fider, classSet } from "@fider/services"
import { PageConfig } from "@fider/components/layouts"
import { UserProfile } from "@fider/components/UserProfile"
import { useStackNavigation } from "@fider/hooks"
import { UserListItem } from "./components"

import "../ManageMembers.page.scss"

export const pageConfig: PageConfig = {
  title: "Members",
  subtitle: "Manage your site administrators and collaborators",
  sidebarItem: "members",
  layoutVariant: "fullWidth",
}

interface ManageMembersPageProps {
  users: User[]
}

interface HistoryState extends Record<string, unknown> {
  userId?: number
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

  const handleStateChange = useCallback(
    (state: HistoryState | null) => {
      if (state?.userId) {
        const user = allUsers.find((u) => u.id === state.userId)
        if (user) {
          setSelectedUser(user)
          setProfileKey((prev) => prev + 1)
        } else {
          setSelectedUser(null)
        }
      } else {
        setSelectedUser(null)
      }
    },
    [allUsers]
  )

  const { pushState, isNavigating } = useStackNavigation<HistoryState>({
    onStateChange: handleStateChange,
    urlPath: "/admin/members",
  })

  const canViewEmails = Fider.session.user.isAdministrator || Fider.session.user.isCollaborator

  const memberFilter = useCallback(
    (searchQuery: string, user: User): boolean => {
      const queryLower = searchQuery.toLowerCase()

      if (user.name.toLowerCase().indexOf(queryLower) >= 0) {
        return true
      }

      if (canViewEmails && user.email && user.email.toLowerCase().indexOf(queryLower) >= 0) {
        return true
      }

      if (user.providers && user.providers.length > 0) {
        for (const provider of user.providers) {
          if (provider.uid.toLowerCase().indexOf(queryLower) >= 0) {
            return true
          }
        }
      }

      return false
    },
    [canViewEmails]
  )

  const handleSearchFilterChanged = useCallback(
    (newQuery: string) => {
      const users = allUsers.filter((x) => memberFilter(newQuery, x)).sort(sortByStaff)
      setQuery(newQuery)
      setFilteredUsers(users)
      setVisibleUsers(users.slice(0, 20))
    },
    [allUsers, memberFilter]
  )

  const showMore = () => {
    setVisibleUsers(filteredUsers.slice(0, visibleUsers.length + 20))
  }

  const clearSearch = () => {
    handleSearchFilterChanged("")
  }

  const handleSelectUser = (user: User) => {
    if (!isNavigating.current) {
      pushState({ userId: user.id })
    }
    setSelectedUser(user)
    setProfileKey((prev) => prev + 1)
  }

  const handleDeselectUser = () => {
    if (!isNavigating.current) {
      pushState({})
    }
    setSelectedUser(null)
  }

  const updateUserInLists = useCallback((updatedUser: Partial<User> & { id: number }) => {
    setAllUsers((prev) => prev.map((u) => (u.id === updatedUser.id ? { ...u, ...updatedUser } : u)))
    setFilteredUsers((prev) => prev.map((u) => (u.id === updatedUser.id ? { ...u, ...updatedUser } : u)))
    setVisibleUsers((prev) => prev.map((u) => (u.id === updatedUser.id ? { ...u, ...updatedUser } : u)))
    setSelectedUser((prev) => (prev && prev.id === updatedUser.id ? { ...prev, ...updatedUser } : prev))
  }, [])

  const searchPlaceholder = canViewEmails ? "Search by name / email / provider..." : "Search by name / provider..."

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

      <div
        className={classSet({
          "c-members-split-view__preview": true,
          "c-members-split-view__preview--mobile-open": selectedUser !== null,
        })}
      >
        {selectedUser ? (
          <>
            <Button
              variant="tertiary"
              size="small"
              className="c-members-split-view__back-btn"
              onClick={handleDeselectUser}
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

