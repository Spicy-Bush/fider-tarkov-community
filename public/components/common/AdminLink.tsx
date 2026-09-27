import React from "react"

interface AdminLinkProps {
  href: string
  className?: string
  title?: string
  children: React.ReactNode
  onClick?: () => void
}

export const AdminLink: React.FC<AdminLinkProps> = ({ href, className, title, children, onClick }) => {
  return (
    <a
      href={href}
      className={className}
      title={title}
      onClick={(event) => {
        if (event.button === 0 && !event.metaKey && !event.ctrlKey && !event.shiftKey && !event.altKey) {
          onClick?.()
        }
      }}
    >
      {children}
    </a>
  )
}
