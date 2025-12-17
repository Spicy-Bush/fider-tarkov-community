import React from "react"
import { HStack } from "@fider/components/layout"

interface FooterLink {
  title: string
  url: string
  external?: boolean
}

interface FooterLinksProps {
  links: FooterLink[]
}

export const FooterLinks = ({ links }: FooterLinksProps) => {
  if (links.length === 0) return null

  return (
    <div className="border-t border-border mt-8 pt-8">
      <HStack spacing={6} justify="center" className="flex-wrap">
        {links.map((link, index) => (
          <a
            key={index}
            href={link.url}
            target={link.external ? "_blank" : undefined}
            rel={link.external ? "noopener noreferrer" : undefined}
            className="text-sm text-muted hover:text-foreground"
          >
            {link.title}
          </a>
        ))}
      </HStack>
    </div>
  )
}
