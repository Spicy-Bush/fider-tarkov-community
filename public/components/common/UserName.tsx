import "./UserName.scss"

import React from "react"
import { isAdministrator, isCollaborator, isHelper, isModerator, UserRole, VisualRole, getVisualRoleName } from "@fider/models"
import { classSet } from "@fider/services"
import { useFider } from "@fider/hooks"

interface UserNameProps {
  user: {
    id: number
    name: string
    role?: UserRole
    visualRole?: VisualRole
    email?: string
  }
  showEmail?: boolean
  clickable?: boolean
}

export const UserName = (props: UserNameProps) => {
  const fider = useFider()
  const isStaff = props.user.role && isCollaborator(props.user.role)
  const isHelp = props.user.role && isHelper(props.user.role)
  const isMod = props.user.role && isModerator(props.user.role)
  const isAdmin = props.user.role && isAdministrator(props.user.role)
  const clickable = props.clickable !== undefined ? props.clickable : true
  
  const getDefaultVisualRole = (): VisualRole => {
    if (!props.user.role) return VisualRole.Visitor;
    
    if (isAdmin) return VisualRole.Administrator;
    if (isMod) return VisualRole.Moderator;
    if (isStaff) return VisualRole.BSGCrew;
    if (isHelp) return VisualRole.Helper;
    return VisualRole.Visitor;
  }
  
  let visualRole = props.user.visualRole || getDefaultVisualRole();
  
  const classNames: Record<string, boolean> = {
    "c-username": true,
  }
  
  if (visualRole) {
    classNames[`vr-${visualRole}`] = true;
  }
  
  const className = classSet(classNames)
  const visualRoleName = getVisualRoleName(visualRole)

  const userName = props.user.name || "Anonymous"
  const userEmail = props.showEmail && props.user.email && (
    <span className="c-username--email">{props.user.email}</span>
  )
  
  const visualRoleSpan = visualRoleName && visualRole !== VisualRole.Visitor && (
    <span className="c-username--visualrole"></span>
  )

  const canViewProfile = fider.session.isAuthenticated && (
    isAdministrator(fider.session.user.role) || 
    isModerator(fider.session.user.role) || 
    isCollaborator(fider.session.user.role)
  )

  if (canViewProfile && props.user.id && clickable) {
    return (
      <>
        <div className={className}>
          <a href={`/profile/${props.user.id}`}>
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
      <div className={className}>
        <span>{userName}</span>
        {visualRoleSpan}
      </div>
      {userEmail}
    </>
  )
}