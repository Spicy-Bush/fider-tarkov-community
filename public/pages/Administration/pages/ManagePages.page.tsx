import React, { useState } from "react"
import { Page, PageTopic, PageTag } from "@fider/models"
import { Button, Input, Modal, Icon } from "@fider/components"
import { HStack, VStack } from "@fider/components/layout"
import { deletePage, createPageTopic, updatePageTopic, deletePageTopic, createPageTag, updatePageTag, deletePageTag } from "@fider/services/pages"
import { notify } from "@fider/services"
import { heroiconsPencilAlt as IconEdit, heroiconsTrash as IconTrash, heroiconsPlus as IconPlus } from "@fider/icons.generated"

interface ManagePagesProps {
  pages: Page[]
  topics: PageTopic[]
  tags: PageTag[]
}

type Tab = "pages" | "configuration"

const ManagePages = ({ pages: initialPages, topics: initialTopics, tags: initialTags }: ManagePagesProps) => {
  const [activeTab, setActiveTab] = useState<Tab>("pages")
  const [pages, setPages] = useState(initialPages)
  const [topics, setTopics] = useState(initialTopics)
  const [tags, setTags] = useState(initialTags)
  const [filter, setFilter] = useState("")

  const [showTopicModal, setShowTopicModal] = useState(false)
  const [showTagModal, setShowTagModal] = useState(false)
  const [editingTopic, setEditingTopic] = useState<PageTopic | null>(null)
  const [editingTag, setEditingTag] = useState<PageTag | null>(null)
  const [topicName, setTopicName] = useState("")
  const [tagName, setTagName] = useState("")

  const handleDelete = async (id: number) => {
    if (!confirm("Are you sure you want to delete this page?")) return

    const result = await deletePage(id)
    if (result.ok) {
      setPages(pages.filter((p) => p.id !== id))
      notify.success("Page deleted")
    } else {
      notify.error("Failed to delete page")
    }
  }

  const openTopicModal = (topic?: PageTopic) => {
    setEditingTopic(topic || null)
    setTopicName(topic?.name || "")
    setShowTopicModal(true)
  }

  const openTagModal = (tag?: PageTag) => {
    setEditingTag(tag || null)
    setTagName(tag?.name || "")
    setShowTagModal(true)
  }

  const handleSaveTopic = async () => {
    if (!topicName.trim()) {
      notify.error("Topic name is required")
      return
    }

    if (editingTopic) {
      const result = await updatePageTopic(editingTopic.id, { name: topicName.trim() })
      if (result.ok) {
        setTopics(topics.map(t => t.id === editingTopic.id ? { ...t, name: topicName.trim() } : t))
        notify.success("Topic updated")
        setShowTopicModal(false)
      } else {
        notify.error("Failed to update topic")
      }
    } else {
      const result = await createPageTopic({ name: topicName.trim() })
      if (result.ok && result.data) {
        setTopics([...topics, result.data])
        notify.success("Topic created")
        setShowTopicModal(false)
      } else {
        notify.error("Failed to create topic")
      }
    }
  }

  const handleDeleteTopic = async (id: number) => {
    if (!confirm("Are you sure you want to delete this topic? Pages using this topic will be unaffected but will no longer show it.")) return

    const result = await deletePageTopic(id)
    if (result.ok) {
      setTopics(topics.filter(t => t.id !== id))
      notify.success("Topic deleted")
    } else {
      notify.error("Failed to delete topic")
    }
  }

  const handleSaveTag = async () => {
    if (!tagName.trim()) {
      notify.error("Tag name is required")
      return
    }

    if (editingTag) {
      const result = await updatePageTag(editingTag.id, { name: tagName.trim() })
      if (result.ok) {
        setTags(tags.map(t => t.id === editingTag.id ? { ...t, name: tagName.trim() } : t))
        notify.success("Tag updated")
        setShowTagModal(false)
      } else {
        notify.error("Failed to update tag")
      }
    } else {
      const result = await createPageTag({ name: tagName.trim() })
      if (result.ok && result.data) {
        setTags([...tags, result.data])
        notify.success("Tag created")
        setShowTagModal(false)
      } else {
        notify.error("Failed to create tag")
      }
    }
  }

  const handleDeleteTag = async (id: number) => {
    if (!confirm("Are you sure you want to delete this tag? Pages using this tag will be unaffected but will no longer show it.")) return

    const result = await deletePageTag(id)
    if (result.ok) {
      setTags(tags.filter(t => t.id !== id))
      notify.success("Tag deleted")
    } else {
      notify.error("Failed to delete tag")
    }
  }

  const filteredPages = pages.filter((page) =>
    page.title.toLowerCase().includes(filter.toLowerCase())
  )

  const getStatusBadge = (status: string) => {
    const classes = {
      published: "bg-success-light text-success-dark border-success-medium",
      draft: "bg-warning-light text-warning-dark border-warning-medium",
      unpublished: "bg-tertiary text-muted border-border",
      scheduled: "bg-info-light text-info-dark border-info-medium",
    }
    return classes[status as keyof typeof classes] || classes.draft
  }

  const tabClass = (tab: Tab) =>
    `px-4 py-2 text-sm font-medium cursor-pointer border-b-2 transition-colors ${
      activeTab === tab
        ? "border-primary text-primary"
        : "border-transparent text-muted hover:text-foreground hover:border-border"
    }`

  return (
    <div className="container p-4">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
        <h1 className="text-display">Manage Pages</h1>
        {activeTab === "pages" && (
          <Button variant="primary" href="/admin/pages/new" size="small">
            Create New Page
          </Button>
        )}
      </div>

      <div className="flex border-b border-border mb-6">
        <button className={tabClass("pages")} onClick={() => setActiveTab("pages")}>
          Pages
        </button>
        <button className={tabClass("configuration")} onClick={() => setActiveTab("configuration")}>
          Configuration
        </button>
      </div>

      {activeTab === "pages" && (
        <>
          <div className="mb-6">
            <input
              type="text"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder="Filter pages..."
              className="w-full px-4 py-2 bg-elevated border border-border rounded-input text-foreground"
            />
          </div>

          <div className="hidden md:block bg-elevated border border-border rounded-panel overflow-hidden">
            <table className="min-w-full">
              <thead className="bg-tertiary">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-muted uppercase tracking-wider">
                    Title
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-muted uppercase tracking-wider">
                    Status
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-muted uppercase tracking-wider">
                    Visibility
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-muted uppercase tracking-wider">
                    Updated
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-muted uppercase tracking-wider">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {filteredPages.map((page) => (
                  <tr key={page.id} className="hover:bg-tertiary">
                    <td className="px-6 py-4">
                      <div className="text-sm font-medium text-foreground">{page.title}</div>
                      <div className="text-sm text-muted">{page.slug}</div>
                    </td>
                    <td className="px-6 py-4">
                      <span
                        className={`px-2 py-1 text-xs font-semibold rounded-badge border ${getStatusBadge(
                          page.status
                        )}`}
                      >
                        {page.status}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-sm text-muted">
                      {page.visibility}
                    </td>
                    <td className="px-6 py-4 text-sm text-muted">
                      {new Date(page.updatedAt).toLocaleDateString()}
                    </td>
                    <td className="px-6 py-4">
                      <HStack spacing={4}>
                        <a href={`/admin/pages/edit/${page.id}`} className="text-link">
                          Edit
                        </a>
                        <a href={`/pages/${page.slug}`} className="text-link" target="_blank">
                          View
                        </a>
                        <button
                          onClick={() => handleDelete(page.id)}
                          className="text-danger hover:text-danger-dark cursor-pointer"
                        >
                          Delete
                        </button>
                      </HStack>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="md:hidden space-y-3">
            {filteredPages.map((page) => (
              <div key={page.id} className="bg-elevated border border-border rounded-card p-4">
                <div className="flex items-start justify-between gap-3 mb-3">
                  <div className="min-w-0 flex-1">
                    <div className="text-sm font-medium text-foreground truncate">{page.title}</div>
                    <div className="text-xs text-muted truncate">{page.slug}</div>
                  </div>
                  <span
                    className={`px-2 py-1 text-xs font-semibold rounded-badge border shrink-0 ${getStatusBadge(
                      page.status
                    )}`}
                  >
                    {page.status}
                  </span>
                </div>
                <div className="flex items-center justify-between text-xs text-muted mb-3">
                  <span>{page.visibility}</span>
                  <span>{new Date(page.updatedAt).toLocaleDateString()}</span>
                </div>
                <div className="flex items-center gap-4 pt-3 border-t border-border">
                  <a href={`/admin/pages/edit/${page.id}`} className="text-sm text-link font-medium">
                    Edit
                  </a>
                  <a href={`/pages/${page.slug}`} className="text-sm text-link" target="_blank">
                    View
                  </a>
                  <button
                    onClick={() => handleDelete(page.id)}
                    className="text-sm text-danger hover:text-danger-dark cursor-pointer ml-auto"
                  >
                    Delete
                  </button>
                </div>
              </div>
            ))}
          </div>

          {filteredPages.length === 0 && (
            <div className="text-center py-12">
              <p className="text-muted">No pages found</p>
            </div>
          )}
        </>
      )}

      {activeTab === "configuration" && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <div className="bg-elevated border border-border rounded-panel p-4">
            <HStack justify="between" className="mb-4">
              <h2 className="text-lg font-semibold">Topics</h2>
              <Button size="small" variant="secondary" onClick={() => openTopicModal()}>
                <Icon sprite={IconPlus} className="h-4 w-4 mr-1" /> Add
              </Button>
            </HStack>
            {topics.length === 0 ? (
              <p className="text-muted text-sm">No topics created yet</p>
            ) : (
              <VStack spacing={2}>
                {topics.map(topic => (
                  <div key={topic.id} className="flex items-center justify-between p-2 bg-tertiary rounded-card">
                    <span className="text-sm">{topic.name}</span>
                    <HStack spacing={2}>
                      <button onClick={() => openTopicModal(topic)} className="text-muted hover:text-foreground cursor-pointer">
                        <Icon sprite={IconEdit} className="h-4 w-4" />
                      </button>
                      <button onClick={() => handleDeleteTopic(topic.id)} className="text-muted hover:text-danger cursor-pointer">
                        <Icon sprite={IconTrash} className="h-4 w-4" />
                      </button>
                    </HStack>
                  </div>
                ))}
              </VStack>
            )}
          </div>

          <div className="bg-elevated border border-border rounded-panel p-4">
            <HStack justify="between" className="mb-4">
              <h2 className="text-lg font-semibold">Tags</h2>
              <Button size="small" variant="secondary" onClick={() => openTagModal()}>
                <Icon sprite={IconPlus} className="h-4 w-4 mr-1" /> Add
              </Button>
            </HStack>
            {tags.length === 0 ? (
              <p className="text-muted text-sm">No tags created yet</p>
            ) : (
              <VStack spacing={2}>
                {tags.map(tag => (
                  <div key={tag.id} className="flex items-center justify-between p-2 bg-tertiary rounded-card">
                    <span className="text-sm">{tag.name}</span>
                    <HStack spacing={2}>
                      <button onClick={() => openTagModal(tag)} className="text-muted hover:text-foreground cursor-pointer">
                        <Icon sprite={IconEdit} className="h-4 w-4" />
                      </button>
                      <button onClick={() => handleDeleteTag(tag.id)} className="text-muted hover:text-danger cursor-pointer">
                        <Icon sprite={IconTrash} className="h-4 w-4" />
                      </button>
                    </HStack>
                  </div>
                ))}
              </VStack>
            )}
          </div>
        </div>
      )}

      <Modal.Window isOpen={showTopicModal} onClose={() => setShowTopicModal(false)}>
        <Modal.Header>
          {editingTopic ? "Edit Topic" : "Add Topic"}
        </Modal.Header>
        <Modal.Content>
          <Input
            field="topicName"
            label="Topic Name"
            value={topicName}
            onChange={setTopicName}
            placeholder="Enter topic name"
          />
        </Modal.Content>
        <Modal.Footer>
          <Button variant="tertiary" onClick={() => setShowTopicModal(false)}>Cancel</Button>
          <Button variant="primary" onClick={handleSaveTopic}>Save</Button>
        </Modal.Footer>
      </Modal.Window>

      <Modal.Window isOpen={showTagModal} onClose={() => setShowTagModal(false)}>
        <Modal.Header>
          {editingTag ? "Edit Tag" : "Add Tag"}
        </Modal.Header>
        <Modal.Content>
          <Input
            field="tagName"
            label="Tag Name"
            value={tagName}
            onChange={setTagName}
            placeholder="Enter tag name"
          />
        </Modal.Content>
        <Modal.Footer>
          <Button variant="tertiary" onClick={() => setShowTagModal(false)}>Cancel</Button>
          <Button variant="primary" onClick={handleSaveTag}>Save</Button>
        </Modal.Footer>
      </Modal.Window>
    </div>
  )
}

export default ManagePages
