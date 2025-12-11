import React from "react"
import { Icon, AdminLink } from "@fider/components"
import { VStack } from "@fider/components/layout"
import { useFider } from "@fider/hooks"
import { useLayout } from "@fider/contexts/LayoutContext"
import { useAdminLayout } from "./context"
import { classSet } from "@fider/services"

import {
  heroiconsChevronUp as IconChevron,
  heroiconsUsers as IconUsers,
  heroiconsFlag as IconFlag,
  heroiconsInbox as IconInbox,
  heroiconsCog as IconCog,
  heroiconsDocumentText as IconDocumentText,
  heroiconsChatAlt2 as IconChat,
  heroiconsTag as IconTag,
  heroiconsLink as IconLink,
  heroiconsAdjustments as IconAdjustments,
  heroiconsLock as IconLock,
  heroiconsEnvelope as IconEnvelope,
  heroiconsKey as IconKey,
  heroiconsCreditCard as IconCreditCard,
  heroiconsPhotograph as IconPhoto,
  heroiconsDownload as IconDownload,
} from "@fider/icons.generated"

interface SidebarItemProps {
  title: string
  href: string
  isActive: boolean
  icon: typeof IconChevron
}

const SidebarItem: React.FC<SidebarItemProps> = ({ title, href, isActive, icon }) => {
  const { toggleSidebar } = useLayout()
  const className = classSet({
    "c-admin-sidebar__item": true,
    "c-admin-sidebar__item--active": isActive,
  })

  const handleClick = () => {
    if (window.innerWidth < 640) {
      toggleSidebar()
    }
  }

  return (
    <AdminLink className={className} href={href} title={title} onClick={handleClick}>
      <Icon sprite={icon} className="c-admin-sidebar__item-icon" />
      <span className="c-admin-sidebar__item-text">{title}</span>
    </AdminLink>
  )
}

export const AdminSidebar: React.FC = () => {
  const fider = useFider()
  const { sidebarOpen, toggleSidebar } = useLayout()
  const { sidebarItem } = useAdminLayout()
  const activeItem = sidebarItem || "general"

  const className = classSet({
    "c-admin-sidebar": true,
    "c-admin-sidebar--collapsed": !sidebarOpen,
  })

  return (
    <aside className={className}>
      <nav className="c-admin-sidebar__nav">
        <VStack spacing={0}>
          <SidebarItem title="Post Queue" href="/admin/queue" isActive={activeItem === "queue"} icon={IconInbox} />
          {(fider.session.user.isModerator || fider.session.user.isCollaborator || fider.session.user.isAdministrator) && (
            <>
              <SidebarItem title="Members" href="/admin/members" isActive={activeItem === "members"} icon={IconUsers} />
              <SidebarItem title="Reports" href="/admin/reports" isActive={activeItem === "reports"} icon={IconFlag} />
            </>
          )}
          {fider.session.user.isCollaborator && (
            <>
              <SidebarItem title="General" href="/admin" isActive={activeItem === "general"} icon={IconCog} />
              <SidebarItem title="Content" href="/admin/content-settings" isActive={activeItem === "content"} icon={IconDocumentText} />
              <SidebarItem title="Responses" href="/admin/responses" isActive={activeItem === "responses"} icon={IconChat} />
              <SidebarItem title="Tags" href="/admin/tags" isActive={activeItem === "tags"} icon={IconTag} />
              <SidebarItem title="Webhooks" href="/admin/webhooks" isActive={activeItem === "webhooks"} icon={IconLink} />
            </>
          )}
          {fider.session.user.isAdministrator && (
            <>
              <SidebarItem title="Advanced" href="/admin/advanced" isActive={activeItem === "advanced"} icon={IconAdjustments} />
              <SidebarItem title="Privacy" href="/admin/privacy" isActive={activeItem === "privacy"} icon={IconLock} />
              <SidebarItem title="Invitations" href="/admin/invitations" isActive={activeItem === "invitations"} icon={IconEnvelope} />
              <SidebarItem title="Authentication" href="/admin/authentication" isActive={activeItem === "authentication"} icon={IconKey} />
              {fider.settings.isBillingEnabled && (
                <SidebarItem title="Billing" href="/admin/billing" isActive={activeItem === "billing"} icon={IconCreditCard} />
              )}
              <SidebarItem title="Files" href="/admin/files" isActive={activeItem === "files"} icon={IconPhoto} />
              <SidebarItem title="Export" href="/admin/export" isActive={activeItem === "export"} icon={IconDownload} />
            </>
          )}
        </VStack>
      </nav>
      <button className="c-admin-sidebar__toggle" onClick={toggleSidebar} title={sidebarOpen ? "Collapse" : "Expand"}>
        <Icon sprite={IconChevron} className={sidebarOpen ? "c-admin-sidebar__toggle-icon" : "c-admin-sidebar__toggle-icon--collapsed"} />
      </button>
    </aside>
  )
}
