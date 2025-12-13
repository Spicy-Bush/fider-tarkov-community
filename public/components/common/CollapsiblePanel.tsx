// CollapsiblePanel converted to Tailwind

import React, { useState, useRef } from "react"
import { classSet } from "@fider/services"

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
      "rounded mb-2": true,
      [className || ""]: !!className
    })}>
      <div 
        className="flex justify-between items-center py-1.5 px-2 cursor-pointer select-none bg-tertiary"
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
        <h3 className="m-0 text-base font-medium flex items-center">
          {icon && <span className="mr-1 inline-flex items-center">{icon}</span>}
          {title}
        </h3>
        <span className="text-sm text-subtle">
          {isOpen ? "▲" : "▼"}
        </span>
      </div>
      <div 
        id={`panel-content-${title.replace(/\s+/g, '-').toLowerCase()}`}
        className="p-2 border-t border-surface-alt"
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