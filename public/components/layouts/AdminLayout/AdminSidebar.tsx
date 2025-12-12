// AdminSidebar converted to Tailwind

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
  heroiconsArchive as IconArchive,
} from "@fider/icons.generated"

interface SidebarItemProps {
  title: string
  href: string
  isActive: boolean
  icon: typeof IconChevron
  collapsed: boolean
}

interface SidebarSectionProps {
  label: string
  children: React.ReactNode
  collapsed: boolean
}

const SidebarItem: React.FC<SidebarItemProps> = ({ title, href, isActive, icon, collapsed }) => {
  const { toggleSidebar } = useLayout()

  const handleClick = () => {
    if (window.innerWidth < 768) {
      toggleSidebar()
    }
  }

  return (
    <AdminLink 
      className={classSet({
        "flex items-center gap-2 px-3 py-2 rounded-button text-sm no-underline mb-1 transition-all duration-50": true,
        "text-muted hover:bg-surface-alt hover:text-foreground": !isActive,
        "bg-accent-light text-primary font-semibold": isActive,
      })} 
      href={href} 
      title={title} 
      onClick={handleClick}
    >
      <Icon sprite={icon} className="w-[18px] h-[18px] shrink-0" />
      <span className={classSet({
        "whitespace-nowrap transition-opacity duration-75": true,
        "opacity-0 w-0 overflow-hidden": collapsed,
      })}>{title}</span>
    </AdminLink>
  )
}

const SidebarSection: React.FC<SidebarSectionProps> = ({ label, children, collapsed }) => {
  return (
    <div className="mb-4 last:mb-0">
      <span className={classSet({
        "block text-[11px] font-semibold text-border-strong uppercase tracking-wide px-3 py-1 mb-1 whitespace-nowrap h-5 transition-opacity duration-75": true,
        "opacity-0": collapsed,
      })}>
        {label}
      </span>
      {children}
    </div>
  )
}

export const AdminSidebar: React.FC = () => {
  const fider = useFider()
  const { sidebarOpen, toggleSidebar } = useLayout()
  const { sidebarItem } = useAdminLayout()
  const activeItem = sidebarItem || "general"

  const isModerator = fider.session.user.isModerator
  const isCollaborator = fider.session.user.isCollaborator
  const isAdministrator = fider.session.user.isAdministrator

  return (
    <aside className={classSet({
      "fixed top-0 left-0 h-screen bg-surface border-r border-surface-alt z-dropdown flex flex-col overflow-hidden transition-all duration-75": true,
      "w-[200px]": sidebarOpen,
      "w-[58px]": !sidebarOpen,
      "max-md:z-sidebar max-md:shadow-xl": true,
      "max-md:-translate-x-full max-md:w-[200px]": !sidebarOpen,
    })}>
      <nav className="flex-1 overflow-y-auto overflow-x-hidden p-4 px-2">
        <VStack spacing={0}>
          <SidebarSection label="Moderation" collapsed={!sidebarOpen}>
            <SidebarItem title="Post Queue" href="/admin/queue" isActive={activeItem === "queue"} icon={IconInbox} collapsed={!sidebarOpen} />
            {(isModerator || isCollaborator || isAdministrator) && (
              <>
                <SidebarItem title="Members" href="/admin/members" isActive={activeItem === "members"} icon={IconUsers} collapsed={!sidebarOpen} />
                <SidebarItem title="Reports" href="/admin/reports" isActive={activeItem === "reports"} icon={IconFlag} collapsed={!sidebarOpen} />
                <SidebarItem title="Archive" href="/admin/archive" isActive={activeItem === "archive"} icon={IconArchive} collapsed={!sidebarOpen} />
              </>
            )}
          </SidebarSection>

          {(isCollaborator || isAdministrator) && (
            <SidebarSection label="Site" collapsed={!sidebarOpen}>
              <SidebarItem title="General" href="/admin" isActive={activeItem === "general"} icon={IconCog} collapsed={!sidebarOpen} />
              <SidebarItem title="Content" href="/admin/content-settings" isActive={activeItem === "content"} icon={IconDocumentText} collapsed={!sidebarOpen} />
              <SidebarItem title="Responses" href="/admin/responses" isActive={activeItem === "responses"} icon={IconChat} collapsed={!sidebarOpen} />
              <SidebarItem title="Tags" href="/admin/tags" isActive={activeItem === "tags"} icon={IconTag} collapsed={!sidebarOpen} />
              <SidebarItem title="Webhooks" href="/admin/webhooks" isActive={activeItem === "webhooks"} icon={IconLink} collapsed={!sidebarOpen} />
            </SidebarSection>
          )}

          {isAdministrator && (
            <SidebarSection label="System" collapsed={!sidebarOpen}>
              <SidebarItem title="Advanced" href="/admin/advanced" isActive={activeItem === "advanced"} icon={IconAdjustments} collapsed={!sidebarOpen} />
              <SidebarItem title="Privacy" href="/admin/privacy" isActive={activeItem === "privacy"} icon={IconLock} collapsed={!sidebarOpen} />
              <SidebarItem title="Invitations" href="/admin/invitations" isActive={activeItem === "invitations"} icon={IconEnvelope} collapsed={!sidebarOpen} />
              <SidebarItem title="Authentication" href="/admin/authentication" isActive={activeItem === "authentication"} icon={IconKey} collapsed={!sidebarOpen} />
              {fider.settings.isBillingEnabled && (
                <SidebarItem title="Billing" href="/admin/billing" isActive={activeItem === "billing"} icon={IconCreditCard} collapsed={!sidebarOpen} />
              )}
              <SidebarItem title="Files" href="/admin/files" isActive={activeItem === "files"} icon={IconPhoto} collapsed={!sidebarOpen} />
              <SidebarItem title="Export" href="/admin/export" isActive={activeItem === "export"} icon={IconDownload} collapsed={!sidebarOpen} />
            </SidebarSection>
          )}
        </VStack>
      </nav>
      <button 
        className="flex items-center justify-center p-3 border-none bg-transparent cursor-pointer border-t border-surface-alt text-border-strong hover:bg-surface-alt hover:text-muted transition-all duration-50" 
        onClick={toggleSidebar} 
        title={sidebarOpen ? "Collapse" : "Expand"}
      >
        <Icon sprite={IconChevron} className={classSet({
          "w-4 h-4 transition-transform duration-75": true,
          "-rotate-90": sidebarOpen,
          "rotate-90": !sidebarOpen,
        })} />
      </button>
    </aside>
  )
}
