// UserProfileTabs converted to Tailwind

import React, { ReactNode, Children, isValidElement } from "react"
import { Icon } from "@fider/components"
import { useUserProfile, ProfileTab } from "./context"
import { Trans } from "@lingui/react/macro"
import { heroiconsSearch as IconSearch, heroiconsExclamation as IconWarning, heroiconsPencilAlt as IconDocument } from "@fider/icons.generated"
import { classSet } from "@fider/services"
import { Tabs, TabPanels } from "@fider/components/common/Tabs"

interface UserProfileTabsProps {
  children: ReactNode
}

export const UserProfileTabs: React.FC<UserProfileTabsProps> = ({ children }) => {
  const { activeTab, setActiveTab, isViewingOwnProfile, isEmbedded } = useUserProfile()
  const tabs: ProfileTab[] = isViewingOwnProfile && !isEmbedded ? ["search", "standing", "settings"] : ["search", "standing"]

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

  const labels = {
    search: <Trans id="profile.tab.search">Search Posts</Trans>,
    standing: <Trans id="profile.tab.standing">Standing</Trans>,
    settings: <Trans id="profile.tab.settings">Settings</Trans>,
  }
  const icons = { search: IconSearch, standing: IconWarning, settings: IconDocument }

  return (
    <div className="grid grid-cols-[200px_minmax(0,1fr)] gap-4 w-full min-h-[400px] max-md:grid-cols-1 max-md:gap-3">
      <Tabs
        tabs={tabs.map((value) => ({
          value,
          label: (
            <>
              <span className="w-4 h-4 flex items-center justify-center shrink-0 max-md:w-3.5 max-md:h-3.5">
                <Icon sprite={icons[value]} className="w-4 h-4 max-md:w-3.5 max-md:h-3.5" />
              </span>
              <span className="max-md:truncate">{labels[value]}</span>
            </>
          ),
        }))}
        activeTab={activeTab}
        onChange={setActiveTab}
        className="w-[200px] sticky top-4 h-fit self-start max-md:static max-md:w-full max-md:pb-2 max-md:mb-1"
        listClassName="flex flex-col gap-1 max-md:flex-row max-md:gap-0"
        indicator="pill"
        indicatorClassName="bg-surface-alt rounded-button"
        tabClassName={(selected) => classSet({
          "flex items-center justify-center gap-1.5 px-3 py-2 bg-transparent border-none rounded-button text-muted text-[0.95em] font-medium cursor-pointer transition-colors duration-150 ease-out text-left w-full max-md:flex-1 max-md:text-xs max-md:px-2 max-md:py-2 max-md:gap-1": true,
          "hover:bg-surface-alt/50 hover:text-foreground": !selected,
          "text-primary": selected,
        })}
      />

      {/* Preserve search results and unsaved settings between tabs. */}
      <TabPanels keys={tabs} activeKey={activeTab} keepMounted className="w-full min-w-0" panelClassName="w-full min-h-[400px]">
        {(tab) => getTabContent(tab)}
      </TabPanels>
    </div>
  )
}
