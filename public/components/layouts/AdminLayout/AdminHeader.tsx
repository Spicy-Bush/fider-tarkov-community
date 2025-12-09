import React from "react"
import { Header, Icon } from "@fider/components"
import { useLayout } from "@fider/contexts/LayoutContext"
import IconMenu from "@fider/assets/images/heroicons-menu.svg"

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
