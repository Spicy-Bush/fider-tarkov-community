import React, { useState, useCallback } from "react"
import { TextArea, Form, Button, ButtonClickEvent, Input } from "@fider/components"
import { Failure, actions, Fider, notify } from "@fider/services"
import { CollapsiblePanel } from "@fider/components/common/CollapsiblePanel"
import { PageConfig } from "@fider/components/layouts"
import { HStack, VStack } from "@fider/components/layout"
import { NavigationLink } from "@fider/models"

type EditableLink = Pick<NavigationLink, "title" | "url" | "displayOrder"> & {
  id?: number
  location: "footer" | "subheader"
  tempId: string
}

interface AdvancedSettingsPageProps {
  customCSS: string
  profanityWords: string
  navigationLinks?: {
    footer: NavigationLink[]
    subheader: NavigationLink[]
  }
}

export const pageConfig: PageConfig = {
  title: "Advanced",
  subtitle: "Manage your site settings",
  sidebarItem: "advanced",
}

const toEditableLinks = (links: NavigationLink[], loc: "footer" | "subheader"): EditableLink[] =>
  links.map((l, i) => ({ ...l, location: loc, tempId: l.id ? `existing-${l.id}` : `new-${i}-${Date.now()}` }))

const AdvancedSettingsPage: React.FC<AdvancedSettingsPageProps> = (props) => {
  const [customCSS, setCustomCSS] = useState(props.customCSS)
  const [profanityWords, setProfanityWords] = useState(props.profanityWords)
  const [footerLinks, setFooterLinks] = useState<EditableLink[]>(() => toEditableLinks(props.navigationLinks?.footer || [], "footer"))
  const [subheaderLinks, setSubheaderLinks] = useState<EditableLink[]>(() => toEditableLinks(props.navigationLinks?.subheader || [], "subheader"))
  const [error, setError] = useState<Failure | undefined>()

  const handleSaveProfanityWords = async (e: ButtonClickEvent): Promise<void> => {
    const result = await actions.updateProfanityWords(profanityWords)
    if (result.ok) {
      e.preventEnable()
      location.reload()
    } else {
      setError(result.error)
      const errorMessage = result.error?.errors?.[0]?.message || "Failed to save profanity words"
      notify.error(errorMessage)
    }
  }

  const handleSaveCustomCSS = async (e: ButtonClickEvent): Promise<void> => {
    const result = await actions.updateTenantAdvancedSettings(customCSS)
    if (result.ok) {
      e.preventEnable()
      location.reload()
    } else {
      setError(result.error)
      const errorMessage = result.error?.errors?.[0]?.message || "Failed to save custom CSS"
      notify.error(errorMessage)
    }
  }

  const addLink = useCallback((loc: "footer" | "subheader") => {
    const newLink: EditableLink = { title: "", url: "", displayOrder: 0, location: loc, tempId: `new-${Date.now()}` }
    if (loc === "footer") setFooterLinks(prev => [...prev, newLink])
    else setSubheaderLinks(prev => [...prev, newLink])
  }, [])

  const removeLink = useCallback((loc: "footer" | "subheader", index: number) => {
    if (loc === "footer") setFooterLinks(prev => prev.filter((_, i) => i !== index))
    else setSubheaderLinks(prev => prev.filter((_, i) => i !== index))
  }, [])

  const updateLink = useCallback((loc: "footer" | "subheader", index: number, field: "title" | "url", value: string) => {
    const update = (links: EditableLink[]) => {
      const updated = [...links]
      updated[index] = { ...updated[index], [field]: value }
      return updated
    }
    if (loc === "footer") setFooterLinks(update)
    else setSubheaderLinks(update)
  }, [])

  const moveLink = useCallback((loc: "footer" | "subheader", index: number, direction: "up" | "down") => {
    const move = (links: EditableLink[]) => {
      const newLinks = [...links]
      const newIndex = direction === "up" ? index - 1 : index + 1
      if (newIndex < 0 || newIndex >= newLinks.length) return links
      ;[newLinks[index], newLinks[newIndex]] = [newLinks[newIndex], newLinks[index]]
      return newLinks.map((link, i) => ({ ...link, displayOrder: i }))
    }
    if (loc === "footer") setFooterLinks(move)
    else setSubheaderLinks(move)
  }, [])

  const handleSaveNavigation = async (e: ButtonClickEvent): Promise<void> => {
    const allLinks = [
      ...footerLinks.map((link, i) => ({ ...link, displayOrder: i, location: "footer" as const })),
      ...subheaderLinks.map((link, i) => ({ ...link, displayOrder: i, location: "subheader" as const })),
    ]
    const result = await actions.saveNavigationLinks(allLinks)
    if (result.ok) {
      e.preventEnable()
      location.reload()
    } else {
      setError(result.error)
      const errorMessage = result.error?.errors?.[0]?.message || "Failed to save navigation links"
      notify.error(errorMessage)
    }
  }

  const renderLinkEditor = (loc: "footer" | "subheader", links: EditableLink[], title: string) => (
    <div className="mb-4">
      <div className="flex items-center justify-between mb-3">
        <h4 className="text-base font-medium">{title}</h4>
        <Button size="small" onClick={() => addLink(loc)}>Add Link</Button>
      </div>
      <VStack spacing={2}>
        {links.length === 0 && <p className="text-muted text-sm">No links configured</p>}
        {links.map((link, index) => (
          <div key={link.tempId} className="bg-surface rounded-card border border-border p-3">
            <HStack spacing={2} className="mb-2">
              <div className="flex-1">
                <Input field={`${loc}-title-${link.tempId}`} placeholder="Link Title" value={link.title} onChange={(v) => updateLink(loc, index, "title", v)} />
              </div>
              <div className="flex-1">
                <Input field={`${loc}-url-${link.tempId}`} placeholder="/pages/about or https://..." value={link.url} onChange={(v) => updateLink(loc, index, "url", v)} />
              </div>
            </HStack>
            <HStack spacing={2} justify="between">
              <HStack spacing={1}>
                <Button size="small" variant="tertiary" onClick={() => moveLink(loc, index, "up")} disabled={index === 0}>Up</Button>
                <Button size="small" variant="tertiary" onClick={() => moveLink(loc, index, "down")} disabled={index === links.length - 1}>Down</Button>
              </HStack>
              <Button size="small" variant="danger" onClick={() => removeLink(loc, index)}>Remove</Button>
            </HStack>
          </div>
        ))}
      </VStack>
    </div>
  )

  return (
    <Form error={error}>
      <CollapsiblePanel title="Navigation Links" defaultOpen={false}>
        <p className="text-muted mb-4">
          Configure footer and collapsible subheader navigation links. Links are displayed in the order shown.
        </p>
        {renderLinkEditor("subheader", subheaderLinks, "Subheader Bar")}
        {renderLinkEditor("footer", footerLinks, "Footer Links")}
        <div className="c-admin-actions">
          <Button variant="primary" onClick={handleSaveNavigation}>
            Save Navigation Links
          </Button>
        </div>
      </CollapsiblePanel>

      <CollapsiblePanel title="Custom CSS" defaultOpen={false}>
        <TextArea
          field="customCSS"
          label="Custom CSS"
          disabled={!Fider.session.user.isAdministrator}
          minRows={10}
          value={customCSS}
          onChange={setCustomCSS}
        >
          {}
        </TextArea>

        <div className="c-admin-actions">
          <Button variant="primary" onClick={handleSaveCustomCSS}>
            Save Custom CSS
          </Button>
        </div>
      </CollapsiblePanel>

      <TextArea
        field="profanityWords"
        label="Profanity Words (one per line)"
        disabled={!Fider.session.user.isAdministrator}
        minRows={5}
        value={profanityWords}
        onChange={setProfanityWords}
      >
        <p className="text-muted">
          Enter banned words, one per line. Any post or comment containing these words will be blocked.
        </p>
      </TextArea>

      <div className="c-admin-actions">
        <Button variant="primary" onClick={handleSaveProfanityWords}>
          Save Profanity Words
        </Button>
      </div>
    </Form>
  )
}

export default AdvancedSettingsPage
