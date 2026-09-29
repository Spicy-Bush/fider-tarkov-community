// UserProfileDetails converted to Tailwind

import React, { useState } from "react"
import { DisplayError, Icon, Select, SelectOption } from "@fider/components"
import { useUserProfile } from "./context"
import { UserRole, VisualRole, userRoleLabels } from "@fider/models"
import { actions } from "@fider/services"
import { ChangedUserRole, ChangedVisualRole } from "@fider/services/actions/tenant"
import { Failure, RequestError } from "@fider/services/http"
import { useFider } from "@fider/hooks"
import { heroiconsChevronDown as IconChevronDown, heroiconsChevronUp as IconChevronUp, heroiconsMail as IconMail, heroiconsIdentification as IconIdentification } from "@fider/icons.generated"

interface UserProfileDetailsProps {
  providers?: { name: string; uid: string }[]
  email?: string
  onRoleChange?: (user: ChangedUserRole) => void
  onVisualRoleChange?: (user: ChangedVisualRole) => void
}

export const UserProfileDetails: React.FC<UserProfileDetailsProps> = ({
  providers,
  email,
  onRoleChange,
  onVisualRoleChange,
}) => {
  const { user } = useUserProfile()
  const { session } = useFider()
  const [isExpanded, setIsExpanded] = useState(false)
  const [isChangingRole, setIsChangingRole] = useState(false)
  const [isChangingVisualRole, setIsChangingVisualRole] = useState(false)
  const [error, setError] = useState<Failure>()

  if (!user) return null

  const canChangeRole = user.permissions.changeRole
  const canChangeVisualRole = user.permissions.changeVisualRole

  const roleOptions: SelectOption[] = Object.entries(userRoleLabels).map(([value, label]) => ({ value, label }))

  const visualRoleOptions: SelectOption[] = [
    { label: "Default", value: "" },
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

  const handleRoleChange = async (option?: SelectOption) => {
    if (!option || !onRoleChange) return

    setError(undefined)
    setIsChangingRole(true)
    try {
      const newRole = option.value as UserRole
      const result = await actions.changeUserRole(user.id, newRole)
      if (result.ok) {
        onRoleChange(result.data)
      } else {
        setError(result.error)
      }
    } catch (cause) {
      if (!(cause instanceof RequestError)) throw cause

      setError({ errors: [{ message: "The role change could not be confirmed. Check your connection and retry." }], cause })
    } finally {
      setIsChangingRole(false)
    }
  }

  const handleVisualRoleChange = async (option?: SelectOption) => {
    if (!option || !onVisualRoleChange) return

    setError(undefined)
    setIsChangingVisualRole(true)
    try {
      const result = await actions.changeUserVisualRole(user.id, option.value as VisualRole)
      if (result.ok) {
        if (session.isAuthenticated && session.user.id === user.id) {
          session.updateUserProfile({
            visualRole: result.data.visualRole,
            visualRoleOverride: result.data.visualRoleOverride,
          })
        }
        onVisualRoleChange(result.data)
      } else {
        setError(result.error)
      }
    } catch (cause) {
      if (!(cause instanceof RequestError)) throw cause

      setError({ errors: [{ message: "The badge change could not be confirmed. Check your connection and retry." }], cause })
    } finally {
      setIsChangingVisualRole(false)
    }
  }

  return (
    <div className="bg-elevated border-b border-surface-alt">
      <button 
        className="w-full flex items-center justify-between p-3 bg-transparent border-none cursor-pointer text-sm font-medium text-muted transition-colors hover:bg-tertiary"
        onClick={() => setIsExpanded(!isExpanded)}
      >
        <span>User Details</span>
        <Icon sprite={isExpanded ? IconChevronUp : IconChevronDown} className="h-4" />
      </button>

      {isExpanded && (
        <div className="px-3 pb-3 flex flex-col gap-2">
          <DisplayError error={error} fields={["", "userID", "visualRole", "role"]} />
          {email && (
            <div className="flex items-center gap-2 text-sm">
              <Icon sprite={IconMail} className="h-4" />
              <span className="text-border-strong font-medium min-w-[80px]">Email:</span>
              <span className="text-foreground">{email}</span>
            </div>
          )}

          {providers && providers.length > 0 && (
            <div className="flex flex-col gap-1">
              <div className="flex items-center gap-2 text-sm">
                <Icon sprite={IconIdentification} className="h-4" />
                <span className="text-border-strong font-medium min-w-[80px]">Provider IDs:</span>
              </div>
              {providers.map((provider, idx) => (
                <div key={idx} className="flex items-center gap-2 pl-6 text-xs">
                  <span className="text-muted capitalize">{provider.name}:</span>
                  <span className="font-mono text-muted bg-surface px-1 py-0.5 rounded">{provider.uid}</span>
                </div>
              ))}
            </div>
          )}

          <div className="flex items-center gap-2 text-sm">
            <span className="text-border-strong font-medium min-w-[80px]">User ID:</span>
            <span className="font-mono text-xs text-muted bg-surface px-1 py-0.5 rounded">{user.id}</span>
          </div>

          {canChangeRole && (
            <div className="flex items-center gap-2 text-sm flex-wrap">
              <span className="text-border-strong font-medium min-w-[80px]">Role:</span>
              <Select
                field="role"
                value={user.role as string}
                options={roleOptions}
                onChange={handleRoleChange}
                disabled={isChangingRole}
              />
            </div>
          )}

          {!canChangeRole && (
            <div className="flex items-center gap-2 text-sm">
              <span className="text-border-strong font-medium min-w-[80px]">Role:</span>
              <span className="text-foreground">{userRoleLabels[user.role as UserRole]}</span>
            </div>
          )}

          {canChangeVisualRole && (
            <div className="flex items-center gap-2 text-sm flex-wrap">
              <span className="text-border-strong font-medium min-w-[80px]">Visual Role:</span>
              <Select
                field="visualRole"
                value={user.visualRoleOverride || ""}
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
