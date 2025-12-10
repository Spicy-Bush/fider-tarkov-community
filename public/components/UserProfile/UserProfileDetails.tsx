import React, { useState } from "react"
import { Icon, Select, SelectOption } from "@fider/components"
import { useUserProfile } from "./context"
import { UserRole, VisualRole } from "@fider/models"
import { actions, Fider } from "@fider/services"
import { heroiconsChevronDown as IconChevronDown, heroiconsChevronUp as IconChevronUp, heroiconsMail as IconMail, heroiconsIdentification as IconIdentification } from "@fider/icons.generated"

interface UserProfileDetailsProps {
  providers?: { name: string; uid: string }[]
  email?: string
  onRoleChange?: (role: UserRole) => void
  onVisualRoleChange?: (visualRole: VisualRole) => void
}

export const UserProfileDetails: React.FC<UserProfileDetailsProps> = ({
  providers,
  email,
  onRoleChange,
  onVisualRoleChange,
}) => {
  const { user } = useUserProfile()
  const [isExpanded, setIsExpanded] = useState(false)
  const [isChangingRole, setIsChangingRole] = useState(false)
  const [isChangingVisualRole, setIsChangingVisualRole] = useState(false)

  if (!user) return null

  const currentUser = Fider.session.user
  const isAdmin = currentUser?.isAdministrator
  const isCollaborator = currentUser?.isCollaborator
  const canChangeRole = isAdmin && currentUser.id !== user.id
  const canChangeVisualRole = (isAdmin || isCollaborator) && currentUser.id !== user.id

  const roleOptions: SelectOption[] = [
    { label: "Visitor", value: UserRole.Visitor },
    { label: "Helper", value: UserRole.Helper },
    { label: "Moderator", value: UserRole.Moderator },
    { label: "Collaborator", value: UserRole.Collaborator },
    { label: "Administrator", value: UserRole.Administrator },
  ]

  const visualRoleOptions: SelectOption[] = [
    { label: "None", value: "" },
    { label: "Visitor", value: "Visitor" },
    { label: "Helper", value: "Helper" },
    { label: "Moderator", value: "Moderator" },
    { label: "Administrator", value: "Administrator" },
    { label: "BSG Crew", value: "BSGCrew" },
    { label: "Developer", value: "Developer" },
    { label: "Sherpa", value: "Sherpa" },
    { label: "TC Staff", value: "TCStaff" },
    { label: "Emissary", value: "Emissary" },
  ]

  const getRoleName = (role: UserRole | number): string => {
    if (role === UserRole.Visitor) return "Visitor"
    if (role === UserRole.Helper) return "Helper"
    if (role === UserRole.Moderator) return "Moderator"
    if (role === UserRole.Collaborator) return "Collaborator"
    if (role === UserRole.Administrator) return "Administrator"
    return "Unknown"
  }

  const handleRoleChange = async (option?: SelectOption) => {
    if (!option || !onRoleChange) return
    setIsChangingRole(true)
    try {
      const newRole = option.value as UserRole
      const result = await actions.changeUserRole(user.id, newRole)
      if (result.ok) {
        onRoleChange(newRole)
      }
    } finally {
      setIsChangingRole(false)
    }
  }

  const handleVisualRoleChange = async (option?: SelectOption) => {
    if (!onVisualRoleChange) return
    setIsChangingVisualRole(true)
    try {
      const visualRoleToNumber: Record<string, number> = {
        "": 0, "Visitor": 1, "Helper": 2, "Administrator": 3, "Moderator": 4,
        "BSGCrew": 5, "Developer": 6, "Sherpa": 7, "TCStaff": 8, "Emissary": 9
      }
      const response = await fetch(`/_api/admin/visualroles/${visualRoleToNumber[option?.value || ""]}/users`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ userID: user.id }),
      })
      if (response.ok) {
        onVisualRoleChange((option?.value || "") as VisualRole)
      }
    } finally {
      setIsChangingVisualRole(false)
    }
  }

  return (
    <div className="c-user-profile__details">
      <button 
        className="c-user-profile__details-toggle"
        onClick={() => setIsExpanded(!isExpanded)}
      >
        <span>User Details</span>
        <Icon sprite={isExpanded ? IconChevronUp : IconChevronDown} className="h-4" />
      </button>

      {isExpanded && (
        <div className="c-user-profile__details-content">
          {email && (
            <div className="c-user-profile__details-row">
              <Icon sprite={IconMail} className="h-4" />
              <span className="c-user-profile__details-label">Email:</span>
              <span className="c-user-profile__details-value">{email}</span>
            </div>
          )}

          {providers && providers.length > 0 && (
            <div className="c-user-profile__details-section">
              <div className="c-user-profile__details-row">
                <Icon sprite={IconIdentification} className="h-4" />
                <span className="c-user-profile__details-label">Provider IDs:</span>
              </div>
              {providers.map((provider, idx) => (
                <div key={idx} className="c-user-profile__details-provider">
                  <span className="c-user-profile__details-provider-name">{provider.name}:</span>
                  <span className="c-user-profile__details-provider-id">{provider.uid}</span>
                </div>
              ))}
            </div>
          )}

          <div className="c-user-profile__details-row">
            <span className="c-user-profile__details-label">User ID:</span>
            <span className="c-user-profile__details-value c-user-profile__details-value--mono">{user.id}</span>
          </div>

          {canChangeRole && (
            <div className="c-user-profile__details-row c-user-profile__details-row--control">
              <span className="c-user-profile__details-label">Role:</span>
              <Select
                field="role"
                defaultValue={user.role as string}
                options={roleOptions}
                onChange={handleRoleChange}
                disabled={isChangingRole}
              />
            </div>
          )}

          {!canChangeRole && (
            <div className="c-user-profile__details-row">
              <span className="c-user-profile__details-label">Role:</span>
              <span className="c-user-profile__details-value">{getRoleName(user.role)}</span>
            </div>
          )}

          {canChangeVisualRole && (
            <div className="c-user-profile__details-row c-user-profile__details-row--control">
              <span className="c-user-profile__details-label">Visual Role:</span>
              <Select
                field="visualRole"
                defaultValue={(user as any).visualRole || ""}
                options={visualRoleOptions}
                onChange={handleVisualRoleChange}
                disabled={isChangingVisualRole}
              />
            </div>
          )}
        </div>
      )}
    </div>
  )
}
