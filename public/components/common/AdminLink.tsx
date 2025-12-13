import React, { useCallback, MouseEvent } from "react"
import { useAdminNavigation } from "@fider/contexts/AdminNavigationContext"

interface AdminLinkProps {
  href: string
  className?: string
  title?: string
  children: React.ReactNode
  onClick?: () => void
}

export const AdminLink: React.FC<AdminLinkProps> = ({ href, className, title, children, onClick }) => {
  const { navigate, prefetch } = useAdminNavigation()

  const handleClick = useCallback(
    (e: MouseEvent<HTMLAnchorElement>) => {
      if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return

      e.preventDefault()
      onClick?.()
      navigate(href)
    },
    [href, navigate, onClick]
  )

  const handleMouseEnter = useCallback(() => {
    prefetch(href)
  }, [href, prefetch])

  return (
    <a
      href={href}
      className={className}
      title={title}
      onClick={handleClick}
      onMouseEnter={handleMouseEnter}
    >
      {children}
    </a>
  )
}
