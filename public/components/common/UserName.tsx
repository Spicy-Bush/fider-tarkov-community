import React from "react"
import { UserPermissions } from "@fider/models"
import { UserRole, VisualRole } from "@fider/models/identity"
import { classSet } from "@fider/services/utils"

interface UserNameProps {
  user: {
    id: number
    name: string
    role?: UserRole
    visualRole?: VisualRole | string
    email?: string
    permissions: Pick<UserPermissions, "readProfile">
  }
  showEmail?: boolean
  clickable?: boolean
}

export const UserName = (props: UserNameProps) => {
  const clickable = props.clickable !== undefined ? props.clickable : true
  const visualRole = props.user.visualRole
  const vrClass = visualRole ? `vr-${visualRole}` : ""

  const userName = props.user.name || "Anonymous"
  const userEmail = props.showEmail && props.user.email && (
    <span className="ml-2.5 text-subtle text-xs font-normal">{props.user.email}</span>
  )

  const visualRoleSpan = visualRole && visualRole !== VisualRole.Visitor && (
    <span className="c-username--visualrole"></span>
  )

  if (props.user.permissions.readProfile && props.user.id && clickable) {
    return (
      <>
        <div className="font-semibold inline-flex items-center">
          <a href={`/profile/${props.user.id}`} className={classSet({ "hover:underline": true, [vrClass]: !!vrClass })}>
            <span>{userName}</span>
            {visualRoleSpan}
          </a>
        </div>
        {userEmail}
      </>
    )
  }

  return (
    <>
      <div className="font-semibold inline-flex items-center">
        <span className={vrClass || "text-foreground"}>{userName}</span>
        {visualRoleSpan}
      </div>
      {userEmail}
    </>
  )
}
