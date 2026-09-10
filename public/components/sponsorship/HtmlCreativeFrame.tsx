import React from "react"

export interface HtmlCreativeFrameProps {
  html: string
  title: string
  className?: string
}

/**
 * Renders ad HTML inside a sandboxed iframe via srcDoc.
 * NEVER use dangerouslySetInnerHTML for ad creatives.
 */
export const HtmlCreativeFrame: React.FC<HtmlCreativeFrameProps> = ({ html, title, className }) => {
  if (!html) return null
  return (
    <iframe
      className={className || "sponsorship-html-frame w-full border-0 rounded bg-surface-alt min-h-[80px]"}
      title={title}
      sandbox="allow-popups allow-popups-to-escape-sandbox allow-scripts"
      srcDoc={html}
      referrerPolicy="no-referrer"
      loading="lazy"
    />
  )
}
