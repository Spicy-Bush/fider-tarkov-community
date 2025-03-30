import React, { useState, useRef } from "react"
import { classSet } from "@fider/services"

import "./CollapsiblePanel.scss"

interface CollapsiblePanelProps {
  title: string
  children: React.ReactNode
  defaultOpen?: boolean
  className?: string
  icon?: React.ReactNode
}

export const CollapsiblePanel: React.FC<CollapsiblePanelProps> = ({ 
  title, 
  children, 
  defaultOpen = false,
  className,
  icon
}) => {
  const [isOpen, setIsOpen] = useState(defaultOpen)
  const contentRef = useRef<HTMLDivElement>(null)

  const togglePanel = () => {
    setIsOpen(!isOpen)
  }

  return (
    <div className={classSet({
      "c-collapsible-panel": true,
      "c-collapsible-panel--active": isOpen,
      [className || ""]: !!className
    })}>
      <div 
        className="c-collapsible-panel-header"
        role="button"
        aria-expanded={isOpen}
        aria-controls={`panel-content-${title.replace(/\s+/g, '-').toLowerCase()}`}
        tabIndex={0}
        onClick={(e) => {
          e.preventDefault();
          e.stopPropagation();
          togglePanel();
        }}
        onKeyPress={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            togglePanel();
          }
        }}
      >
        <h3>
          {icon && <span className="c-collapsible-panel-icon">{icon}</span>}
          {title}
        </h3>
        <span className="c-collapsible-panel-arrow">
          {isOpen ? "▲" : "▼"}
        </span>
      </div>
      <div 
        id={`panel-content-${title.replace(/\s+/g, '-').toLowerCase()}`}
        className="c-collapsible-panel-content"
        aria-hidden={!isOpen}
        ref={contentRef}
        style={{ 
          display: isOpen ? 'block' : 'none'
        }}
      >
        {isOpen && children}
      </div>
    </div>
  )
}