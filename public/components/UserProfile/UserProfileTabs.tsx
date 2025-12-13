// UserProfileTabs converted to Tailwind

import React, { ReactNode, Children, isValidElement } from "react"
import { Icon } from "@fider/components"
import { useUserProfile, ProfileTab } from "./context"
import { Trans } from "@lingui/react/macro"
import { heroiconsSearch as IconSearch, heroiconsExclamation as IconWarning, heroiconsPencilAlt as IconDocument } from "@fider/icons.generated"
import { classSet } from "@fider/services"

interface UserProfileTabsProps {
  children: ReactNode
}

export const UserProfileTabs: React.FC<UserProfileTabsProps> = ({ children }) => {
  const { activeTab, setActiveTab, isViewingOwnProfile, isEmbedded } = useUserProfile()

  const getTabContent = (tab: ProfileTab): ReactNode => {
    const childArray = Children.toArray(children)
    
    for (const child of childArray) {
      if (isValidElement(child)) {
        const displayName = (child.type as any).displayName || (child.type as any).name
        if (tab === "search" && displayName === "UserProfileSearch") return child
        if (tab === "standing" && displayName === "UserProfileStanding") return child
        if (tab === "settings" && displayName === "UserProfileSettings") return child
      }
    }
    return null
  }

  const handleTabChange = (tab: ProfileTab) => {
    setActiveTab(tab)
  }

  return (
    <div className="grid grid-cols-[200px_minmax(0,1fr)] gap-4 w-full min-h-[400px] max-md:grid-cols-1 max-md:gap-3">
      <div className="flex flex-col gap-1 w-[200px] sticky top-4 h-fit self-start max-md:static max-md:w-full max-md:flex-row max-md:gap-0 max-md:pb-2 max-md:mb-1">
        <button 
          className={classSet({
            "flex items-center justify-center gap-1.5 px-3 py-2 bg-transparent border-none rounded-button text-muted text-[0.95em] font-medium cursor-pointer transition-all text-left w-full max-md:flex-1 max-md:text-xs max-md:px-2 max-md:py-2 max-md:gap-1": true,
            "hover:bg-surface-alt hover:text-foreground": activeTab !== "search",
            "bg-surface-alt text-primary": activeTab === "search",
          })}
          onClick={() => handleTabChange("search")}
        >
          <span className="w-4 h-4 flex items-center justify-center shrink-0 max-md:w-3.5 max-md:h-3.5">
            <Icon sprite={IconSearch} className="w-4 h-4 max-md:w-3.5 max-md:h-3.5" />
          </span>
          <span className="max-md:truncate">
            <Trans id="profile.tab.search">Search Posts</Trans>
          </span>
        </button>
        <button 
          className={classSet({
            "flex items-center justify-center gap-1.5 px-3 py-2 bg-transparent border-none rounded-button text-muted text-[0.95em] font-medium cursor-pointer transition-all text-left w-full max-md:flex-1 max-md:text-xs max-md:px-2 max-md:py-2 max-md:gap-1": true,
            "hover:bg-surface-alt hover:text-foreground": activeTab !== "standing",
            "bg-surface-alt text-primary": activeTab === "standing",
          })}
          onClick={() => handleTabChange("standing")}
        >
          <span className="w-4 h-4 flex items-center justify-center shrink-0 max-md:w-3.5 max-md:h-3.5">
            <Icon sprite={IconWarning} className="w-4 h-4 max-md:w-3.5 max-md:h-3.5" />
          </span>
          <span className="max-md:truncate">
            <Trans id="profile.tab.standing">Standing</Trans>
          </span>
        </button>
        {isViewingOwnProfile && !isEmbedded && (
          <button 
            className={classSet({
              "flex items-center justify-center gap-1.5 px-3 py-2 bg-transparent border-none rounded-button text-muted text-[0.95em] font-medium cursor-pointer transition-all text-left w-full max-md:flex-1 max-md:text-xs max-md:px-2 max-md:py-2 max-md:gap-1": true,
              "hover:bg-surface-alt hover:text-foreground": activeTab !== "settings",
              "bg-surface-alt text-primary": activeTab === "settings",
            })}
            onClick={() => handleTabChange("settings")}
          >
            <span className="w-4 h-4 flex items-center justify-center shrink-0 max-md:w-3.5 max-md:h-3.5">
              <Icon sprite={IconDocument} className="w-4 h-4 max-md:w-3.5 max-md:h-3.5" />
            </span>
            <span className="max-md:truncate">
              <Trans id="profile.tab.settings">Settings</Trans>
            </span>
          </button>
        )}
      </div>

      <div className={activeTab === "search" ? "block w-full min-h-[400px]" : "hidden w-full min-h-[400px]"}>
        {getTabContent("search")}
      </div>

      <div className={activeTab === "standing" ? "block w-full min-h-[400px]" : "hidden w-full min-h-[400px]"}>
        {getTabContent("standing")}
      </div>

      {isViewingOwnProfile && !isEmbedded && (
        <div className={activeTab === "settings" ? "block w-full min-h-[400px]" : "hidden w-full min-h-[400px]"}>
          {getTabContent("settings")}
        </div>
      )}
    </div>
  )
}
