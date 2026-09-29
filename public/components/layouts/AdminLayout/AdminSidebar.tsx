// AdminSidebar converted to Tailwind

import React from "react"
import { Icon } from "@fider/components/common/Icon"
import { AdminLink } from "@fider/components/common/AdminLink"
import { VStack } from "@fider/components/layout/Stack"
import { useFider } from "@fider/hooks/use-fider"
import { useLayout } from "@fider/contexts/LayoutContext"
import { useAdminLayout } from "./context"
import { classSet } from "@fider/services/utils"

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
  heroiconsSpeakerphone as IconSpeaker,
  permissionsMatrix as IconPermissions,
  sheetExport as IconSheetExport,
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

  const permissions = fider.session.permissions
  const sections = [
    {
      label: "Moderation",
      items: [
        { title: "Post Queue", path: "queue", icon: IconInbox, visible: permissions.manageQueue },
        { title: "Members", path: "members", icon: IconUsers, visible: permissions.manageMembers },
        { title: "Reports", path: "reports", icon: IconFlag, visible: permissions.manageReports },
        { title: "Archive", path: "archive", icon: IconArchive, visible: permissions.manageArchive },
        { title: "BSG Export", path: "bsg-export", icon: IconSheetExport, visible: permissions.exportFeedback },
      ],
    },
    {
      label: "Site",
      items: [
        { title: "General", path: "", icon: IconCog, visible: permissions.readSettings },
        { title: "Content", path: "content-settings", icon: IconDocumentText, visible: permissions.manageContentSettings },
        { title: "Pages", path: "pages", icon: IconDocumentText, visible: permissions.managePages },
        { title: "Responses", path: "responses", icon: IconChat, visible: permissions.manageResponses },
        { title: "Tags", path: "tags", icon: IconTag, visible: permissions.manageTags },
        { title: "Sponsorship", path: "sponsorship", icon: IconSpeaker, visible: permissions.manageSponsorship },
        { title: "Webhooks", path: "webhooks", icon: IconLink, visible: permissions.manageWebhooks },
      ],
    },
    {
      label: "System",
      items: [
        { title: "Advanced", path: "advanced", icon: IconAdjustments, visible: permissions.manageSettings },
        { title: "Privacy", path: "privacy", icon: IconLock, visible: permissions.manageSettings },
        { title: "Invitations", path: "invitations", icon: IconEnvelope, visible: permissions.manageInvitations },
        { title: "Authentication", path: "authentication", icon: IconKey, visible: permissions.manageAuthentication },
        { title: "Permissions", path: "permissions", icon: IconPermissions, visible: permissions.manageRolePermissions },
        { title: "Billing", path: "billing", icon: IconCreditCard, visible: permissions.manageBilling && fider.settings.isBillingEnabled },
        { title: "Files", path: "files", icon: IconPhoto, visible: permissions.manageFiles },
        { title: "Export", path: "export", icon: IconDownload, visible: permissions.exportFeedback },
      ],
    },
  ]

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
          {sections.map((section) => {
            const items = section.items.filter((item) => item.visible)
            if (items.length === 0) return null

            return (
              <SidebarSection key={section.label} label={section.label} collapsed={!sidebarOpen}>
                {items.map((item) => (
                  <SidebarItem
                    key={item.path}
                    title={item.title}
                    href={item.path ? "/admin/" + item.path : "/admin"}
                    isActive={activeItem === (item.path === "content-settings" ? "content" : item.path || "general")}
                    icon={item.icon}
                    collapsed={!sidebarOpen}
                  />
                ))}
              </SidebarSection>
            )
          })}
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
