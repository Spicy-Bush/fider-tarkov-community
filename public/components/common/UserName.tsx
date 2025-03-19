import "./UserName.scss"

import React from "react"
import { isAdministrator, isCollaborator, isModerator, UserRole, VisualRole, getVisualRoleName } from "@fider/models"
import { classSet } from "@fider/services"

interface UserNameProps {
  user: {
    id: number
    name: string
    role?: UserRole
    visualRole?: VisualRole
    email?: string
  }
  showEmail?: boolean
}

export const UserName = (props: UserNameProps) => {
  const isStaff = props.user.role && isCollaborator(props.user.role)
  const isMod = props.user.role && isModerator(props.user.role)
  const isAdmin = props.user.role && isAdministrator(props.user.role)
  
  const getDefaultVisualRole = (): VisualRole => {
    if (!props.user.role) return VisualRole.Visitor
    
    if (isAdmin) return VisualRole.Administrator
    if (isMod) return VisualRole.Moderator
    if (isStaff) return VisualRole.BSGCrew
    return VisualRole.Visitor
  }
  
  let visualRole: VisualRole = VisualRole.None
  
  if (props.user.visualRole !== undefined && props.user.visualRole !== 0) {
    visualRole = props.user.visualRole
  } else {
    visualRole = getDefaultVisualRole()
  }
  
  const classNames: Record<string, boolean> = {
    "c-username": true,
  }
  
  if (isAdmin) {
    classNames["c-username--Admin"] = true
  } else if (isMod) {
    classNames["c-username--Moderator"] = true
  } else if (isStaff) {
    classNames["c-username--BSG"] = true
  }
  
  if (visualRole !== VisualRole.None) {
    classNames[`vr-${visualRole}`] = true
  }
  
  const className = classSet(classNames)
  const visualRoleName = getVisualRoleName(visualRole)

  console.log("User visual role:", visualRole);
  console.log("Class names:", classNames);

  return (
    <div className={className}>
      <span>{props.user.name || "Anonymous"}</span>
      <>{props.showEmail && props.user.email && <span className="c-username--email">({props.user.email})</span>}</>

      {isStaff && (
        <div data-tooltip={isStaff ? "Staff" : undefined}>
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
            <path
              fillRule="evenodd"
              d="M6.267 3.455a3.066 3.066 0 001.745-.723 3.066 3.066 0 013.976 0 3.066 3.066 0 001.745.723 3.066 3.066 0 012.812 2.812c.051.643.304 1.254.723 1.745a3.066 3.066 0 010 3.976 3.066 3.066 0 00-.723 1.745 3.066 3.066 0 01-2.812 2.812 3.066 3.066 0 00-1.745.723 3.066 3.066 0 01-3.976 0 3.066 3.066 0 00-1.745-.723 3.066 3.066 0 01-2.812-2.812 3.066 3.066 0 00-.723-1.745 3.066 3.066 0 010-3.976 3.066 3.066 0 00.723-1.745 3.066 3.066 0 012.812-2.812zm7.44 5.252a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
              clipRule="evenodd"
            ></path>
          </svg>
        </div>
      )}
      
      {visualRoleName && visualRole !== VisualRole.Visitor && (
        <span className="c-username--visualrole">({visualRoleName})</span>
      )}
    </div>
  )
}