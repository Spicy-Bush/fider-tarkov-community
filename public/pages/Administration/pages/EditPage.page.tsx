import React, { useState, useRef, useCallback, useEffect, lazy, Suspense } from "react"
import { useFider } from "@fider/hooks/use-fider"
import { usePageCollaboration } from "@fider/hooks/usePageCollaboration"
import { DraftPicker } from "@fider/components/common/DraftPicker"
import { Button, DisplayError, ImagePicker, Input, Modal, TextArea, Icon } from "@fider/components"
import { Page, PageTopic, PageTag, PageDraft, User } from "@fider/models"
import { Failure, http, markdown, uploadedImageURL } from "@fider/services"
import { newSubmissionID } from "@fider/services/postSubmission"
import { requestOutcome } from "@fider/services/http"
import { PageConfig } from "@fider/components/layouts"
import { PageContentDocsPanel } from "../components/page/PageContentDocsPanel"
import { HStack } from "@fider/components/layout"
import { heroiconsArrowUpDown as IconSync } from "@fider/icons.generated"

const CollaborativePageContent = lazy(() => import("../components/page/CollaborativePageContent"))

export const pageConfig: PageConfig = {
  title: "Edit Page",
  subtitle: "Create or edit a page",
  sidebarItem: "pages",
  layoutVariant: "fullWidth",
}

interface EditPagePageProps {
  page?: Page
  draft?: PageDraft
  topics: PageTopic[]
  tags: PageTag[]
  roles: string[]
  users: User[]
}

const EditPagePage = (props: EditPagePageProps) => {
  const fider = useFider()
  const account = `${fider.session.tenant.id}:${fider.session.isAuthenticated ? fider.session.user.id : "anonymous"}`
  return <EditPageEditor key={`${account}:${props.page?.id || "new"}`} {...props} />
}

const EditPageEditor = (props: EditPagePageProps) => {
  const isNew = !props.page
  const editorRef = useRef<HTMLElement | null>(null)
  const previewRef = useRef<HTMLDivElement>(null)
  const isSyncing = useRef(false)
  
  const draft = usePageCollaboration(props.page)
  const {
    title, slug, content, excerpt, bannerImage, status, visibility, allowedRoles,
    allowComments, allowCommentImages, allowReactions, scheduledFor,
    showToc: showTOC, topics: selectedTopics, tags: selectedTags, authors: selectedAuthors,
  } = draft.value
  const bannerFile = bannerImage?.kind === "local" ? bannerImage.file : undefined
  const [bannerFileURL, setBannerFileURL] = useState<string>()
  useEffect(() => {
    if (!bannerFile) {
      setBannerFileURL(undefined)
      return
    }
    const url = URL.createObjectURL(bannerFile)
    setBannerFileURL(url)
    return () => URL.revokeObjectURL(url)
  }, [bannerFile])

  const bannerPreview = bannerImage?.kind === "stored" ? uploadedImageURL(bannerImage.bkey) : bannerFileURL
  const [syncScroll, setSyncScroll] = useState(true)
  const [showExcerpt, setShowExcerpt] = useState(!!props.page?.excerpt || !!props.draft?.excerpt)
  const [error, setError] = useState<Failure>()
  const [isSaving, setIsSaving] = useState(false)
  const [publication, setPublication] = useState<{ submissionId: string; status: string; uncertain: boolean }>()
  const [showSettings, setShowSettings] = useState(false)
  const [showDocs, setShowDocs] = useState(false)
  const [showPreview, setShowPreview] = useState(false)
  
  const [availableTopics, setAvailableTopics] = useState<PageTopic[]>(props.topics || [])
  const [availableTags, setAvailableTags] = useState<PageTag[]>(props.tags || [])
  const [newTopicName, setNewTopicName] = useState("")
  const [newTagName, setNewTagName] = useState("")
  const [isCreatingTopic, setIsCreatingTopic] = useState(false)
  const [isCreatingTag, setIsCreatingTag] = useState(false)

  const handleSave = async (newStatus?: string) => {
    setIsSaving(true)
    setError(undefined)

    let requested = false
    try {
      await draft.flush()
      const request = publication || { submissionId: newSubmissionID(), status: newStatus || status, uncertain: false }
      setPublication(request)
      requested = true
      const result = await http.post<Page>(`/api/pages/${draft.session!.pageId}/draft/publish`, {
        submissionId: request.submissionId,
        status: request.status,
      })
      if (result.ok) {
        window.location.href = `/admin/pages/edit/${result.data.id}`
      } else {
        if (requestOutcome(result) === "rejected" && !request.uncertain) {
          setPublication(undefined)
        } else {
          setPublication({ ...request, uncertain: true })
        }
        setError(result.error)
      }
    } catch (cause) {
      if (requested) {
        setPublication(current => current && { ...current, uncertain: true })
        setError({ errors: [{ message: "Could not confirm publication. Retry publishing to check the result." }], cause })
      }
    } finally {
      setIsSaving(false)
    }
  }

  const renderMarkdown = (text: string) => {
    if (!text) return ""
    return markdown.full(text, true)
  }

  const handleEditorScroll = useCallback(() => {
    if (!syncScroll || isSyncing.current || !editorRef.current || !previewRef.current) return
    isSyncing.current = true
    const editor = editorRef.current
    const preview = previewRef.current
    const scrollRatio = editor.scrollTop / (editor.scrollHeight - editor.clientHeight || 1)
    preview.scrollTop = scrollRatio * (preview.scrollHeight - preview.clientHeight)
    requestAnimationFrame(() => { isSyncing.current = false })
  }, [syncScroll])

  const handlePreviewScroll = useCallback(() => {
    if (!syncScroll || isSyncing.current || !editorRef.current || !previewRef.current) return
    isSyncing.current = true
    const editor = editorRef.current
    const preview = previewRef.current
    const scrollRatio = preview.scrollTop / (preview.scrollHeight - preview.clientHeight || 1)
    editor.scrollTop = scrollRatio * (editor.scrollHeight - editor.clientHeight)
    requestAnimationFrame(() => { isSyncing.current = false })
  }, [syncScroll])

  const toggleTopic = (id: number) => {
    draft.change(value => ({ ...value, topics: value.topics.includes(id) ? value.topics.filter(topic => topic !== id) : [...value.topics, id] }))
  }

  const toggleTag = (id: number) => {
    draft.change(value => ({ ...value, tags: value.tags.includes(id) ? value.tags.filter(tag => tag !== id) : [...value.tags, id] }))
  }

  const toggleRole = (role: string) => {
    draft.change(value => ({ ...value, allowedRoles: value.allowedRoles.includes(role) ? value.allowedRoles.filter(item => item !== role) : [...value.allowedRoles, role] }))
  }

  const toggleAuthor = (id: number) => {
    draft.change(value => ({ ...value, authors: value.authors.includes(id) ? value.authors.filter(author => author !== id) : [...value.authors, id] }))
  }

  const createTopic = async () => {
    if (!newTopicName.trim()) return
    setIsCreatingTopic(true)
    try {
      const result = await http.post<PageTopic>("/api/page-topics", { name: newTopicName.trim() })
      if (result.ok && result.data) {
        setAvailableTopics(prev => [...prev, result.data!])
        draft.change(value => ({ ...value, topics: [...value.topics, result.data.id] }))
        setNewTopicName("")
      }
    } finally {
      setIsCreatingTopic(false)
    }
  }

  const createTag = async () => {
    if (!newTagName.trim()) return
    setIsCreatingTag(true)
    try {
      const result = await http.post<PageTag>("/api/page-tags", { name: newTagName.trim() })
      if (result.ok && result.data) {
        setAvailableTags(prev => [...prev, result.data!])
        draft.change(value => ({ ...value, tags: [...value.tags, result.data.id] }))
        setNewTagName("")
      }
    } finally {
      setIsCreatingTag(false)
    }
  }

  const errors = [
    ...(error?.errors || []),
    ...(draft.error?.errors || []),
    ...(draft.recoveryError ? [{ message: draft.recoveryError }] : []),
    ...(bannerImage?.kind === "missing" ? [{ message: `Select ${bannerImage.fileName} again or remove it.` }] : []),
  ]

  return (
    <>
      {draft.alternatives.length > 0 && (
        <div className="mb-4">
          <DraftPicker drafts={draft.alternatives} label="Previous Page drafts" action="Restore draft" disabled={!draft.session} onSelect={draft.restore} />
        </div>
      )}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-4 pb-4 border-b border-border">
        <div className="flex items-center gap-4">
          {!isNew && (
            <a
              href={`/pages/${props.page!.slug}`}
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm text-link hover:text-link-hover"
            >
              View Page
            </a>
          )}
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          <Button variant="tertiary" size="small" onClick={() => setShowDocs(true)}>
            Docs
          </Button>
          <Button variant="tertiary" size="small" disabled={!draft.session} onClick={() => setShowSettings(true)}>
            Settings
          </Button>
          <Button 
            variant="tertiary" 
            size="small" 
            onClick={() => setShowPreview(!showPreview)}
            className="lg:hidden"
          >
            {showPreview ? "Edit" : "Preview"}
          </Button>
          {(draft.error || draft.recoveryError) && <Button variant="secondary" size="small" onClick={draft.retry}>Retry</Button>}
          <Button variant="primary" size="small" onClick={() => handleSave(status === "scheduled" || status === "unpublished" ? status : "published")} disabled={isSaving || !draft.session}>
            {publication ? "Retry publishing" : status === "scheduled" ? "Schedule" : status === "unpublished" ? "Unpublish" : status === "published" ? "Update" : "Publish"}
          </Button>
        </div>
      </div>

      <div className="mb-3">
        <DisplayError error={{ errors }} fields={[...new Set(errors.map(item => item.field || ""))]} />
      </div>

      <fieldset disabled={!draft.session} className="contents">
      <div className="flex flex-col lg:flex-row gap-4 h-[calc(100vh-220px)] sm:h-[calc(100vh-200px)] min-h-[400px]">
        <div className={`flex-1 flex flex-col border border-border rounded-card bg-surface overflow-hidden ${showPreview ? "hidden lg:flex" : "flex"}`}>
          <div className="p-3 sm:p-4 space-y-3 sm:space-y-4 shrink-0">
            <div className="flex flex-col sm:flex-row gap-3 sm:gap-4">
              <div className="flex-2">
                <Input
                  field="title"
                  placeholder="Page Title"
                  value={title}
                  onChange={title => draft.change({ title })}
                  noMargin
                />
              </div>
              <div className="flex-1">
                <Input
                  field="slug"
                  placeholder="url-slug"
                  value={slug}
                  onChange={slug => draft.change({ slug })}
                  noMargin
                />
              </div>
            </div>

            <div>
              <label className="text-sm font-medium mb-2 block">Banner Image</label>
              {bannerImage?.kind === "missing" && (
                <Button onClick={() => draft.change({ bannerImage: null })}>Remove image</Button>
              )}
              <ImagePicker
                field="bannerImage"
                image={bannerPreview}
                onSelect={file => {
                  const id = newSubmissionID()
                  draft.change({ bannerImage: { kind: "local", fileId: id, file } })
                }}
                onRemove={() => draft.change({ bannerImage: bannerImage?.kind === "stored" ? { kind: "removed", bkey: bannerImage.bkey } : null })}
              />
            </div>
          </div>

          <div className="flex-1 flex flex-col min-h-0 px-3 sm:px-4">
            <label className="text-sm font-medium mb-2 block shrink-0">Content</label>
            {draft.session ? (
              <Suspense fallback={<div className="min-h-[200px] flex-1" />}>
                <CollaborativePageContent
                document={draft.session.document}
                awareness={draft.session.awareness}
                scrollElement={editorRef}
                onScroll={handleEditorScroll}
                />
              </Suspense>
            ) : (
              <textarea
                value={content}
                readOnly
                aria-label="Page content"
                placeholder="Write your page content in Markdown..."
                className="flex-1 w-full min-h-[200px] rounded-card border border-border bg-surface p-3 font-mono text-sm"
              />
            )}
          </div>

          <div className="px-3 sm:px-4 pb-3 sm:pb-4 shrink-0">
            <button
              type="button"
              onClick={() => setShowExcerpt(!showExcerpt)}
              className="flex items-center gap-2 text-sm text-muted hover:text-foreground mb-2"
            >
              <span className={`transition-transform ${showExcerpt ? "rotate-90" : ""}`}>&#9654;</span>
              Excerpt
              {excerpt && !showExcerpt && <span className="text-xs text-success">(set)</span>}
            </button>
            {showExcerpt && (
              <TextArea
                field="excerpt"
                placeholder="Brief description for listings"
                value={excerpt}
                onChange={excerpt => draft.change({ excerpt })}
                minRows={2}
              />
            )}
          </div>
        </div>

        <div className={`flex-1 flex flex-col border border-border rounded-card bg-elevated overflow-hidden ${showPreview ? "flex" : "hidden lg:flex"}`}>
          <div className="px-3 sm:px-4 py-3 border-b border-border shrink-0 bg-surface flex items-center justify-between">
            <h2 className="text-base font-medium">Preview</h2>
            <button
              type="button"
              onClick={() => setSyncScroll(!syncScroll)}
              className={`hidden lg:inline-flex items-center gap-1.5 px-2 py-1 text-xs rounded-button border transition-colors ${
                syncScroll 
                  ? "bg-primary text-primary-foreground border-primary" 
                  : "bg-surface text-muted border-border hover:bg-tertiary hover:text-foreground"
              }`}
              title={syncScroll ? "Disable scroll sync" : "Enable scroll sync"}
            >
              <Icon sprite={IconSync} className="w-3.5 h-3.5" />
              Sync
            </button>
          </div>
          <div ref={previewRef} onScroll={handlePreviewScroll} className="flex-1 overflow-y-auto p-4 sm:p-6">
            <div className="relative mb-4">
              {bannerPreview && (
                <img
                  src={bannerPreview}
                  alt={title}
                  className="w-full h-16 object-cover"
                  style={{ maskImage: "linear-gradient(to bottom, black 0%, transparent 100%)", WebkitMaskImage: "linear-gradient(to bottom, black 0%, transparent 100%)" }}
                />
              )}
              <h1 
                className="text-large"
                style={{ marginTop: bannerPreview ? "-0.8rem" : "0" }}
              >
                {title || "Untitled Page"}
              </h1>
            </div>
            <div
              className="c-markdown max-w-none"
              dangerouslySetInnerHTML={{ __html: renderMarkdown(content) }}
            />
          </div>
        </div>
      </div>

      </fieldset>

      <Modal.Window isOpen={showSettings} onClose={() => setShowSettings(false)} size="large">
        <Modal.Header>
          <h2>Page Settings</h2>
        </Modal.Header>
        <Modal.Content>
          <div className="space-y-6">
            <div>
              <label className="text-sm font-medium mb-2 block">Status</label>
              <select
                value={status}
                onChange={(e) => draft.change({ status: e.target.value })}
                className="w-full p-2 rounded-card border border-border bg-surface text-foreground"
              >
                <option value="draft">Draft</option>
                <option value="published">Published</option>
                <option value="unpublished">Unpublished</option>
                <option value="scheduled">Scheduled</option>
              </select>
            </div>

            {status === "scheduled" && (
              <div className="space-y-3">
                <label className="text-sm font-medium block">Publish Date & Time</label>
                <div className="flex gap-3">
                  <div className="flex-1">
                    <label className="text-xs text-muted mb-1 block">Date</label>
                    <input
                      type="date"
                      value={scheduledFor ? scheduledFor.split("T")[0] : ""}
                      onChange={(e) => {
                        const time = scheduledFor ? scheduledFor.split("T")[1] || "12:00" : "12:00"
                        draft.change({ scheduledFor: e.target.value ? `${e.target.value}T${time}` : "" })
                      }}
                      className="w-full p-2 rounded-card border border-border bg-surface text-foreground"
                    />
                  </div>
                  <div className="w-32">
                    <label className="text-xs text-muted mb-1 block">Time</label>
                    <input
                      type="time"
                      value={scheduledFor ? scheduledFor.split("T")[1]?.slice(0, 5) || "12:00" : ""}
                      onChange={(e) => {
                        const date = scheduledFor ? scheduledFor.split("T")[0] : new Date().toISOString().split("T")[0]
                        draft.change({ scheduledFor: e.target.value ? `${date}T${e.target.value}` : "" })
                      }}
                      className="w-full p-2 rounded-card border border-border bg-surface text-foreground"
                    />
                  </div>
                </div>
              </div>
            )}

            <div>
              <label className="text-sm font-medium mb-2 block">Visibility</label>
              <select
                value={visibility}
                onChange={(e) => draft.change({ visibility: e.target.value })}
                className="w-full p-2 rounded-card border border-border bg-surface text-foreground"
              >
                <option value="public">Public</option>
                <option value="private">Private (Role-based)</option>
                <option value="unlisted">Unlisted</option>
              </select>
            </div>

            {visibility === "private" && (
              <div>
                <label className="text-sm font-medium mb-2 block">Allowed Roles</label>
                <div className="space-y-2">
                  {props.roles.map((role) => (
                    <label key={role} className="flex items-center gap-2 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={allowedRoles.includes(role)}
                        onChange={() => toggleRole(role)}
                        className="h-4 w-4 appearance-none border border-border rounded-badge bg-elevated cursor-pointer checked:border-transparent checked:bg-primary checked:bg-size-[100%_100%] checked:bg-center checked:bg-no-repeat"
                        style={{
                          backgroundImage: allowedRoles.includes(role) ? "url(\"data:image/svg+xml,%3csvg viewBox='0 0 16 16' fill='white' xmlns='http://www.w3.org/2000/svg'%3e%3cpath d='M12.207 4.793a1 1 0 010 1.414l-5 5a1 1 0 01-1.414 0l-2-2a1 1 0 011.414-1.414L6.5 9.086l4.293-4.293a1 1 0 011.414 0z'/%3e%3c/svg%3e\")" : "none"
                        }}
                      />
                      <span className="text-sm capitalize">{role}</span>
                    </label>
                  ))}
                </div>
              </div>
            )}

            <div>
              <label className="text-sm font-medium mb-2 block">Authors</label>
              <p className="text-xs text-muted mb-2">Select users to display as authors of this page</p>
              <div className="space-y-2">
                {props.users
                  .map((user) => (
                    <label key={user.id} className="flex items-center gap-2 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={selectedAuthors.includes(user.id)}
                        onChange={() => toggleAuthor(user.id)}
                        className="h-4 w-4 appearance-none border border-border rounded-badge bg-elevated cursor-pointer checked:border-transparent checked:bg-primary checked:bg-size-[100%_100%] checked:bg-center checked:bg-no-repeat"
                        style={{
                          backgroundImage: selectedAuthors.includes(user.id) ? "url(\"data:image/svg+xml,%3csvg viewBox='0 0 16 16' fill='white' xmlns='http://www.w3.org/2000/svg'%3e%3cpath d='M12.207 4.793a1 1 0 010 1.414l-5 5a1 1 0 01-1.414 0l-2-2a1 1 0 011.414-1.414L6.5 9.086l4.293-4.293a1 1 0 011.414 0z'/%3e%3c/svg%3e\")" : "none"
                        }}
                      />
                      <span className="text-sm">{user.name}</span>
                    </label>
                  ))}
                {props.users.length === 0 && (
                  <span className="text-sm text-muted">No eligible users available</span>
                )}
              </div>
            </div>

            <div>
              <label className="text-sm font-medium mb-2 block">Topics</label>
              <div className="flex flex-wrap gap-2 mb-3">
                {availableTopics.map((topic) => (
                  <button
                    key={topic.id}
                    type="button"
                    onClick={() => toggleTopic(topic.id)}
                    className={`px-3 py-1.5 rounded-badge text-sm ${
                      selectedTopics.includes(topic.id)
                        ? "bg-primary text-primary-foreground"
                        : "bg-tertiary text-foreground hover:bg-tertiary-hover"
                    }`}
                  >
                    {topic.name}
                  </button>
                ))}
                {availableTopics.length === 0 && (
                  <span className="text-sm text-muted">No topics yet</span>
                )}
              </div>
              <HStack spacing={2}>
                <input
                  type="text"
                  value={newTopicName}
                  onChange={(e) => setNewTopicName(e.target.value)}
                  placeholder="New topic name..."
                  className="flex-1 px-3 py-1.5 text-sm rounded-input border border-border bg-surface text-foreground"
                  onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), createTopic())}
                />
                <Button
                  variant="secondary"
                  size="small"
                  onClick={createTopic}
                  disabled={isCreatingTopic || !newTopicName.trim()}
                >
                  Add
                </Button>
              </HStack>
            </div>

            <div>
              <label className="text-sm font-medium mb-2 block">Tags</label>
              <div className="flex flex-wrap gap-2 mb-3">
                {availableTags.map((tag) => (
                  <button
                    key={tag.id}
                    type="button"
                    onClick={() => toggleTag(tag.id)}
                    className={`px-3 py-1.5 rounded-badge text-sm ${
                      selectedTags.includes(tag.id)
                        ? "bg-primary text-primary-foreground"
                        : "bg-tertiary text-foreground hover:bg-tertiary-hover"
                    }`}
                  >
                    {tag.name}
                  </button>
                ))}
                {availableTags.length === 0 && (
                  <span className="text-sm text-muted">No tags yet</span>
                )}
              </div>
              <HStack spacing={2}>
                <input
                  type="text"
                  value={newTagName}
                  onChange={(e) => setNewTagName(e.target.value)}
                  placeholder="New tag name..."
                  className="flex-1 px-3 py-1.5 text-sm rounded-input border border-border bg-surface text-foreground"
                  onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), createTag())}
                />
                <Button
                  variant="secondary"
                  size="small"
                  onClick={createTag}
                  disabled={isCreatingTag || !newTagName.trim()}
                >
                  Add
                </Button>
              </HStack>
            </div>

            <div className="space-y-3">
              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={allowComments}
                  onChange={(e) => draft.change({ allowComments: e.target.checked })}
                  className="rounded"
                />
                <span className="text-sm">Allow Comments</span>
              </label>

              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={allowCommentImages}
                  onChange={(event) => draft.change({ allowCommentImages: event.target.checked })}
                  className="rounded"
                />
                <span className="text-sm">Allow Images in Comments</span>
              </label>

              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={allowReactions}
                  onChange={(e) => draft.change({ allowReactions: e.target.checked })}
                  className="rounded"
                />
                <span className="text-sm">Allow Reactions</span>
              </label>

              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={showTOC}
                  onChange={(e) => draft.change({ showToc: e.target.checked })}
                  className="rounded"
                />
                <span className="text-sm">Show Table of Contents</span>
              </label>
            </div>

          </div>

        </Modal.Content>
        <Modal.Footer align="right">
          <Button variant="tertiary" onClick={() => setShowSettings(false)}>
            Close
          </Button>
        </Modal.Footer>
      </Modal.Window>

      <PageContentDocsPanel isOpen={showDocs} onClose={() => setShowDocs(false)} />
    </>
  )
}

export default EditPagePage
