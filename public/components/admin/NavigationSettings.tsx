import React, { useState } from "react"
import { Button } from "@fider/components"
import { VStack, HStack } from "@fider/components/layout"
import { notify } from "@fider/services"

interface NavigationLink {
  title: string
  url: string
  external: boolean
}

interface NavigationSettingsProps {
  footerLinks?: NavigationLink[]
  subheaderPages?: number[]
}

export const NavigationSettings = ({ footerLinks = [], subheaderPages = [] }: NavigationSettingsProps) => {
  const [links, setLinks] = useState<NavigationLink[]>(footerLinks)
  const [newLink, setNewLink] = useState({ title: "", url: "", external: false })

  const addLink = () => {
    if (!newLink.title || !newLink.url) {
      notify.error("Please fill in all fields")
      return
    }
    setLinks([...links, newLink])
    setNewLink({ title: "", url: "", external: false })
  }

  const removeLink = (index: number) => {
    setLinks(links.filter((_, i) => i !== index))
  }

  const handleSave = async () => {
    notify.success("Navigation settings updated")
  }

  return (
    <div className="bg-elevated border border-border rounded-panel p-6">
      <h2 className="text-header mb-4">Footer Links</h2>

      <VStack spacing={4} className="mb-6">
        {links.map((link, index) => (
          <HStack key={index} spacing={4} className="p-4 bg-tertiary rounded-card">
            <div className="flex-1">
              <div className="font-medium text-foreground">{link.title}</div>
              <div className="text-sm text-muted">{link.url}</div>
            </div>
            <Button
              variant="danger"
              size="small"
              onClick={() => removeLink(index)}
            >
              Remove
            </Button>
          </HStack>
        ))}
      </VStack>

      <div className="border-t border-border pt-4">
        <h3 className="text-subtitle mb-3">Add New Link</h3>
        <div className="grid grid-cols-1 md:grid-cols-[1fr_1fr_auto] gap-4">
          <input
            type="text"
            placeholder="Title"
            value={newLink.title}
            onChange={(e) => setNewLink({ ...newLink, title: e.target.value })}
            className="px-4 py-2 bg-surface-alt border border-border rounded-input text-foreground"
          />
          <input
            type="text"
            placeholder="URL"
            value={newLink.url}
            onChange={(e) => setNewLink({ ...newLink, url: e.target.value })}
            className="px-4 py-2 bg-surface-alt border border-border rounded-input text-foreground"
          />
          <HStack spacing={2}>
            <label className="flex items-center">
              <input
                type="checkbox"
                checked={newLink.external}
                onChange={(e) => setNewLink({ ...newLink, external: e.target.checked })}
                className="mr-2"
              />
              <span className="text-sm text-foreground">External</span>
            </label>
            <Button variant="primary" onClick={addLink}>
              Add
            </Button>
          </HStack>
        </div>
      </div>

      <div className="mt-6 pt-6 border-t border-border">
        <Button variant="primary" onClick={handleSave}>
          Save Navigation Settings
        </Button>
      </div>
    </div>
  )
}
