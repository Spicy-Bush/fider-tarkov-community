import "./Tabs.scss"
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
    <div className={`c-tabs ${className || ""}`}>
      <div className="c-tabs-list">
        {tabs.map((tab) => (
          <div
            key={tab.value}
            className={classSet({
              "c-tabs-item": true,
              "is-active": activeTab === tab.value,
            })}
            onClick={() => handleTabClick(tab.value)}
          >
            <span className="c-tabs-item-label">{tab.label}</span>
            {tab.counter !== undefined && (
              <span id="c-dropdown-buttoncount" className="c-tabs-item-counter">{tab.counter}</span>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}