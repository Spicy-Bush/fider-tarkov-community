// AvatarStack converted to Tailwind

import React from "react"
import { UserRole } from "@fider/models"
import { Avatar } from "./Avatar"
import { classSet } from "@fider/services"

interface AvatarStackProps {
  overlap?: boolean
  users: Array<{
    role?: UserRole
    avatarURL: string
    name: string
  }>
}

export const AvatarStack = (props: AvatarStackProps) => {
  const shouldOverlap = props.overlap ?? true

  return (
    <div className="flex">
      {props.users.map((x, i) => (
        <div 
          key={i} 
          className={classSet({
            "border-2 border-white rounded-full": true,
            "-ml-3": shouldOverlap && i > 0,
          })}
        >
          <Avatar user={x} />
        </div>
      ))}
    </div>
  )
}
