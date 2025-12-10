import React, { ReactNode, Children, isValidElement } from "react"
import { Icon } from "@fider/components"
import { useUserProfile, ProfileTab } from "./context"
import { Trans } from "@lingui/react/macro"
import { heroiconsSearch as IconSearch, heroiconsExclamation as IconWarning, heroiconsPencilAlt as IconDocument } from "@fider/icons.generated"

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
    <div className="c-user-profile__sidebar">
      <div className="c-user-profile__nav">
        <button 
          className={`c-user-profile__nav-button ${activeTab === "search" ? "active" : ""}`}
          onClick={() => handleTabChange("search")}
        >
          <span className="icon">
            <Icon sprite={IconSearch} width="16" height="16" />
          </span>
          <Trans id="profile.tab.search">Search Posts</Trans>
        </button>
        <button 
          className={`c-user-profile__nav-button ${activeTab === "standing" ? "active" : ""}`}
          onClick={() => handleTabChange("standing")}
        >
          <span className="icon">
            <Icon sprite={IconWarning} width="16" height="16" />
          </span>
          <Trans id="profile.tab.standing">Standing</Trans>
        </button>
        {isViewingOwnProfile && !isEmbedded && (
          <button 
            className={`c-user-profile__nav-button ${activeTab === "settings" ? "active" : ""}`}
            onClick={() => handleTabChange("settings")}
          >
            <span className="icon">
              <Icon sprite={IconDocument} width="16" height="16" />
            </span>
            <Trans id="profile.tab.settings">Settings</Trans>
          </button>
        )}
      </div>

      <div className={`c-user-profile__content ${activeTab === "search" ? "active" : ""}`}>
        {getTabContent("search")}
      </div>

      <div className={`c-user-profile__content ${activeTab === "standing" ? "active" : ""}`}>
        {getTabContent("standing")}
      </div>

      {isViewingOwnProfile && !isEmbedded && (
        <div className={`c-user-profile__content ${activeTab === "settings" ? "active" : ""}`}>
          {getTabContent("settings")}
        </div>
      )}
    </div>
  )
}

