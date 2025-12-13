import React from "react"
import { UserRole, isAdministrator, isModerator, isCollaborator } from "@fider/models"
import { useFider } from "@fider/hooks"

interface AvatarProps {
  user: {
    id?: number
    role?: UserRole
    avatarURL: string
    name: string
  }
  size?: "small" | "normal" | "fill"
  imageSize?: number
  clickable?: boolean
  className?: string
}

export const Avatar = (props: AvatarProps) => {
  const fider = useFider()
  const sizeClass = props.size === "small" ? "h-6 w-6" : props.size === "fill" ? "w-full h-full" : "h-8 w-8"
  const imageSize = props.imageSize ?? 50
  const clickable = props.clickable !== undefined ? props.clickable : true
  const avatar = (
    <img 
      className={`rounded-full align-middle inline-block aspect-square object-cover ${sizeClass} ${props.className || ""}`} 
      alt={props.user.name} 
      src={`${props.user.avatarURL}?size=${imageSize}`} 
    />
  )
  
  const canViewProfile = fider.session.isAuthenticated && (
    isAdministrator(fider.session.user.role) || 
    isModerator(fider.session.user.role) || 
    isCollaborator(fider.session.user.role)
  )

  if (canViewProfile && props.user.id && clickable) {
    return <a href={`/profile/${props.user.id}`} className={props.size === "fill" ? "w-full h-full block" : ""}>{avatar}</a>
  }
  
  return avatar
}
