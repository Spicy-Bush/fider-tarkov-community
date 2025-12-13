// Tabs converted to Tailwind

import React from "react"
import { classSet } from "@fider/services"

interface TabItem {
  value: string
  label: React.ReactNode
  counter?: number
}

interface TabsProps {
  tabs: TabItem[]
  activeTab: string
  onChange: (value: string) => void
  className?: string
}

export const Tabs: React.FC<TabsProps> = (props) => {
  const { tabs, activeTab, onChange, className } = props

  const handleTabClick = (value: string) => {
    if (value !== activeTab) {
      onChange(value)
    }
  }

  return (
    <div className={`w-full ${className || ""}`}>
      <div className="flex border-b border-surface-alt">
        {tabs.map((tab) => {
          const isActive = activeTab === tab.value
          return (
            <div
              key={tab.value}
              className={classSet({
                "py-2.5 px-4 mr-2 cursor-pointer flex items-center font-medium transition-all duration-75 border-b-2": true,
                "text-subtle hover:text-foreground border-transparent": !isActive,
                "text-primary border-primary": isActive,
              })}
              onClick={() => handleTabClick(tab.value)}
            >
              <span>{tab.label}</span>
              {tab.counter !== undefined && (
                <span 
                  className={classSet({
                    "ml-1.5 py-0.5 px-1.5 rounded-badge text-[11px] min-w-5 text-center": true,
                    "bg-surface-alt text-muted": !isActive,
                    "bg-accent-light text-primary-hover": isActive,
                  })}
                >
                  {tab.counter}
                </span>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}