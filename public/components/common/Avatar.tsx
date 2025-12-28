import React from "react"
import { UserRole, UserAvatarType, isAdministrator, isModerator, isCollaborator } from "@fider/models"
import { useFider } from "@fider/hooks"

const AVATAR_COLORS = [
  { bg: "linear-gradient(135deg, #86b0bc 0%, #6a9aa8 100%)", text: "#ffffff" },
  { bg: "linear-gradient(135deg, #5a9a6b 0%, #4d8a5e 100%)", text: "#ffffff" },
  { bg: "linear-gradient(135deg, #c49a4b 0%, #b08a40 100%)", text: "#ffffff" },
  { bg: "linear-gradient(135deg, #c45a5a 0%, #a84a4a 100%)", text: "#ffffff" },
  { bg: "linear-gradient(135deg, #8a7abc 0%, #7a6aa8 100%)", text: "#ffffff" },
  { bg: "linear-gradient(135deg, #5a8ac4 0%, #4a7ab0 100%)", text: "#ffffff" },
  { bg: "linear-gradient(135deg, #bc8a86 0%, #a87a76 100%)", text: "#ffffff" },
  { bg: "linear-gradient(135deg, #6abcb0 0%, #5aa8a0 100%)", text: "#ffffff" },
]

const getInitials = (name: string): string => {
  const parts = name.trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return "?"
  if (parts.length === 1) return parts[0].charAt(0).toUpperCase()
  return (parts[0].charAt(0) + parts[parts.length - 1].charAt(0)).toUpperCase()
}

const getAvatarColor = (name: string): { bg: string; text: string } => {
  let hash = 0
  for (let i = 0; i < name.length; i++) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash)
  }
  return AVATAR_COLORS[Math.abs(hash) % AVATAR_COLORS.length]
}

const isCustomAvatar = (avatarURL: string | undefined, avatarType?: UserAvatarType): boolean => {
  if (avatarType === UserAvatarType.Custom || avatarType === UserAvatarType.Gravatar) return true
  if (avatarType === UserAvatarType.Letter || !avatarURL) return false
  return avatarURL.includes("/static/images/") || avatarURL.includes("/static/avatars/gravatar/")
}

interface AvatarProps {
  user: {
    id?: number
    role?: UserRole
    avatarURL?: string
    avatarType?: UserAvatarType
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
  const imageSize = props.imageSize ?? 64
  const clickable = props.clickable !== undefined ? props.clickable : true
  
  const useImage = isCustomAvatar(props.user.avatarURL, props.user.avatarType)
  
  const avatar = useImage ? (
    <img 
      className={`rounded-full align-middle inline-block aspect-square object-cover ${sizeClass} ${props.className || ""}`} 
      alt={props.user.name} 
      src={`${props.user.avatarURL}?size=${imageSize}`} 
    />
  ) : (
    <InitialsAvatar name={props.user.name} sizeClass={sizeClass} className={props.className} />
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

interface InitialsAvatarProps {
  name: string
  sizeClass: string
  className?: string
}

const InitialsAvatar: React.FC<InitialsAvatarProps> = ({ name, sizeClass, className = "" }) => {
  const initials = getInitials(name)
  const color = getAvatarColor(name)
  
  return (
    <div 
      className={`rounded-full align-middle inline-flex items-center justify-center aspect-square shrink-0 overflow-hidden ${sizeClass} ${className}`}
      style={{ background: color.bg }}
    >
      <svg viewBox="0 0 100 100" className="w-full h-full">
        <text
          x="50"
          y="50"
          dominantBaseline="central"
          textAnchor="middle"
          fill={color.text}
          fontSize="42"
          fontWeight="700"
          fontFamily="inherit"
        >
          {initials}
        </text>
      </svg>
    </div>
  )
}

export { getInitials, getAvatarColor, AVATAR_COLORS }
