import React from "react"
import { Header, Icon } from "@fider/components"
import { useLayout } from "@fider/contexts/LayoutContext"
import { heroiconsMenu as IconMenu } from "@fider/icons.generated"

export const AdminHeader: React.FC = () => {
  const { toggleSidebar } = useLayout()

  return (
    <div className="c-admin-header">
      <button className="c-admin-header__mobile-nav" onClick={toggleSidebar}>
        <Icon sprite={IconMenu} />
      </button>
      <Header />
    </div>
  )
}
