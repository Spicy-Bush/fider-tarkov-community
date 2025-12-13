// AdminHeader converted to Tailwind

import React from "react"
import { Header, Icon } from "@fider/components"
import { useLayout } from "@fider/contexts/LayoutContext"
import { heroiconsMenu as IconMenu } from "@fider/icons.generated"

export const AdminHeader: React.FC = () => {
  const { toggleSidebar } = useLayout()

  return (
    <div className="bg-elevated border-b border-surface-alt relative">
      <button 
        className="flex md:hidden fixed right-4 bottom-4 p-3 border-none bg-primary cursor-pointer rounded-full text-white z-50 shadow-lg transition-all duration-50 hover:bg-primary-hover hover:scale-105 active:scale-95" 
        onClick={toggleSidebar}
      >
        <Icon sprite={IconMenu} className="w-6 h-6" />
      </button>
      <Header />
    </div>
  )
}
