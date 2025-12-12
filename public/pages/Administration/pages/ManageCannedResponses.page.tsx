import React, { useState, useEffect, useMemo } from "react"
import { 
  Button, 
  Form, 
  Loader, 
  Input, 
  TextArea, 
  Select, 
  SelectOption, 
  Modal, 
  Icon,
  Toggle,
} from "@fider/components"
import { HStack } from "@fider/components/layout"
import { actions, Failure, notify, classSet } from "@fider/services"
import { heroiconsPlusCircle as IconPlus, heroiconsPencilAlt as IconEdit, heroiconsTrash as IconTrash, heroiconsMenu as IconMenu } from "@fider/icons.generated"
import { CannedResponse } from "@fider/services/actions/response"
import { ReportReason } from "@fider/models"
import { Trans } from "@lingui/react/macro"
import { i18n } from "@lingui/core"
import { PageConfig } from "@fider/components/layouts"

export const pageConfig: PageConfig = {
  title: "Responses",
  subtitle: "Manage predefined responses and report reasons",
  sidebarItem: "responses",
}

type TabType = "canned" | "reasons"

const DEFAULT_RESPONSE_TYPES = ["warning", "mute"]

interface ManageCannedResponsesPageProps {
  types: string[]
}

interface ResponseEditorState {
  isOpen: boolean
  id?: number
  type: string
  title: string
  content: string
  duration: string
  isActive: boolean
  error?: Failure
}

interface ReasonEditorState {
  isOpen: boolean
  id?: number
  title: string
  description: string
  isActive: boolean
  error?: Failure
}

const ManageCannedResponsesPage: React.FC<ManageCannedResponsesPageProps> = (props) => {
  const [activeTab, setActiveTab] = useState<TabType>("canned")
  
  const safeTypes = useMemo(() => {
    if (!props.types || !Array.isArray(props.types) || props.types.length === 0) {
      return DEFAULT_RESPONSE_TYPES;
    }
    return props.types;
  }, [props.types]);
    
  const [selectedType, setSelectedType] = useState<string>(safeTypes[0])
  const [responses, setResponses] = useState<CannedResponse[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string>()
  const [editorState, setEditorState] = useState<ResponseEditorState>({
    isOpen: false,
    type: safeTypes[0],
    title: "",
    content: "",
    duration: "",
    isActive: true
  })
  const [deleteConfirmation, setDeleteConfirmation] = useState<{ isOpen: boolean; id?: number }>({
    isOpen: false
  })

  const [reasons, setReasons] = useState<ReportReason[]>([])
  const [isLoadingReasons, setIsLoadingReasons] = useState(false)
  const [hasLoadedReasons, setHasLoadedReasons] = useState(false)
  const [reasonsError, setReasonsError] = useState<string>()
  const [reasonEditorState, setReasonEditorState] = useState<ReasonEditorState>({
    isOpen: false,
    title: "",
    description: "",
    isActive: true,
  })
  const [reasonDeleteConfirmation, setReasonDeleteConfirmation] = useState<{ isOpen: boolean; id?: number }>({
    isOpen: false,
  })
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)
  const [draggedReasonIndex, setDraggedReasonIndex] = useState<number | null>(null)
  const [dragOverReasonIndex, setDragOverReasonIndex] = useState<number | null>(null)

  useEffect(() => {
    loadResponses()
  }, [selectedType])

  useEffect(() => {
    if (activeTab === "reasons" && !hasLoadedReasons && !isLoadingReasons) {
      loadReasons()
    }
  }, [activeTab, hasLoadedReasons, isLoadingReasons])

  const loadReasons = async () => {
    setIsLoadingReasons(true)
    setReasonsError(undefined)

    const result = await actions.listAllReportReasons()
    if (result.ok) {
      setReasons(result.data || [])
    } else {
      setReasonsError("Failed to load report reasons")
    }
    setIsLoadingReasons(false)
    setHasLoadedReasons(true)
  }

  const openCreateReasonEditor = () => {
    setReasonEditorState({
      isOpen: true,
      title: "",
      description: "",
      isActive: true,
    })
  }

  const openEditReasonEditor = (reason: ReportReason) => {
    setReasonEditorState({
      isOpen: true,
      id: reason.id,
      title: reason.title,
      description: reason.description || "",
      isActive: reason.isActive,
    })
  }

  const closeReasonEditor = () => {
    setReasonEditorState((prev) => ({ ...prev, isOpen: false }))
  }

  const handleReasonEditorSubmit = async () => {
    const { id, title, description, isActive } = reasonEditorState

    if (!title.trim()) {
      setReasonEditorState((prev) => ({
        ...prev,
        error: { errors: [{ field: "title", message: "Title is required" }] },
      }))
      return
    }

    setIsSubmitting(true)
    let result: { ok: boolean; error?: Failure }
    if (id) {
      result = await actions.updateReportReason(id, {
        title,
        description,
        isActive,
      })
    } else {
      result = await actions.createReportReason({
        title,
        description,
      })
    }

    if (result.ok) {
      notify.success(
        id
          ? i18n._("reportReasons.updated.success", { message: "Report reason updated successfully" })
          : i18n._("reportReasons.created.success", { message: "Report reason created successfully" })
      )
      closeReasonEditor()
      loadReasons()
    } else if (result.error) {
      setReasonEditorState((prev) => ({ ...prev, error: result.error }))
    }
    setIsSubmitting(false)
  }

  const confirmDeleteReason = (id: number) => {
    setReasonDeleteConfirmation({ isOpen: true, id })
  }

  const handleDeleteReason = async () => {
    const { id } = reasonDeleteConfirmation
    if (id) {
      setIsDeleting(true)
      const result = await actions.deleteReportReason(id)
      if (result.ok) {
        notify.success(i18n._("reportReasons.deleted.success", { message: "Report reason deleted successfully" }))
        setReasonDeleteConfirmation({ isOpen: false })
        loadReasons()
      } else {
        notify.error(i18n._("reportReasons.deleted.error", { message: "Failed to delete report reason" }))
      }
      setIsDeleting(false)
    }
  }

  const handleReasonDragStart = (e: React.DragEvent, index: number) => {
    setDraggedReasonIndex(index)
    e.dataTransfer.effectAllowed = "move"
  }

  const handleReasonDragOver = (e: React.DragEvent, index: number) => {
    e.preventDefault()
    e.dataTransfer.dropEffect = "move"
    setDragOverReasonIndex(index)
  }

  const handleReasonDragLeave = () => {
    setDragOverReasonIndex(null)
  }

  const handleReasonDrop = async (e: React.DragEvent, dropIndex: number) => {
    e.preventDefault()
    if (draggedReasonIndex === null || draggedReasonIndex === dropIndex) {
      setDraggedReasonIndex(null)
      setDragOverReasonIndex(null)
      return
    }

    const newReasons = [...reasons]
    const [draggedItem] = newReasons.splice(draggedReasonIndex, 1)
    newReasons.splice(dropIndex, 0, draggedItem)

    setReasons(newReasons)
    setDraggedReasonIndex(null)
    setDragOverReasonIndex(null)

    const ids = newReasons.map((r) => r.id)
    const result = await actions.reorderReportReasons(ids)
    if (!result.ok) {
      notify.error(i18n._("reportReasons.reorder.error", { message: "Failed to reorder report reasons" }))
      loadReasons()
    }
  }

  const handleReasonDragEnd = () => {
    setDraggedReasonIndex(null)
    setDragOverReasonIndex(null)
  }

  const loadResponses = async () => {
    setIsLoading(true)
    setError(undefined)

    const result = await actions.listCannedResponses(selectedType)
    if (result.ok) {
      setResponses(result.data || [])
    } else {
      setError("Failed to load responses")
    }
    setIsLoading(false)
  }

  const handleTypeChange = (option?: SelectOption) => {
    if (option) {
      setSelectedType(option.value)
    }
  }

  const openCreateEditor = () => {
    setEditorState({
      isOpen: true,
      type: selectedType,
      title: "",
      content: "",
      duration: "",
      isActive: true
    })
  }

  const openEditEditor = (response: CannedResponse) => {
    setEditorState({
      isOpen: true,
      id: response.id,
      type: response.type,
      title: response.title,
      content: response.content,
      duration: response.duration || "",
      isActive: response.isActive
    })
  }

  const closeEditor = () => {
    setEditorState(prev => ({ ...prev, isOpen: false }))
  }

  const handleEditorSubmit = async () => {
    const { id, type, title, content, duration, isActive } = editorState

    if (!title.trim()) {
      setEditorState(prev => ({
        ...prev,
        error: { errors: [{ field: "title", message: "Title is required" }] }
      }))
      return
    }

    if (!content.trim()) {
      setEditorState(prev => ({
        ...prev,
        error: { errors: [{ field: "content", message: "Content is required" }] }
      }))
      return
    }

    setIsSubmitting(true)
    let result: { ok: boolean; error?: Failure }
    if (id) {
      result = await actions.updateCannedResponse(id, { 
        type, title, content, duration, isActive 
      })
    } else {
      result = await actions.createCannedResponse({ 
        type, title, content, duration 
      })
    }

    if (result.ok) {
      notify.success(id 
        ? i18n._("responses.updated.success", { message: "Response updated successfully" })
        : i18n._("responses.created.success", { message: "Response created successfully" })
      )
      closeEditor()
      loadResponses()
    } else if (result.error) {
      setEditorState(prev => ({ ...prev, error: result.error }))
    }
    setIsSubmitting(false)
  }

  const confirmDelete = (id: number) => {
    setDeleteConfirmation({ isOpen: true, id })
  }

  const handleDelete = async () => {
    const { id } = deleteConfirmation
    if (id) {
      setIsDeleting(true)
      const result = await actions.deleteCannedResponse(id)
      if (result.ok) {
        notify.success(i18n._("responses.deleted.success", { message: "Response deleted successfully" }))
        setDeleteConfirmation({ isOpen: false })
        loadResponses()
      } else {
        notify.error(i18n._("responses.deleted.error", { message: "Failed to delete response" }))
      }
      setIsDeleting(false)
    }
  }

  const typeOptions = useMemo(() => safeTypes.map(type => ({
    value: type,
    label: type.charAt(0).toUpperCase() + type.slice(1)
  })), [safeTypes])

  const tabClass = (tab: TabType) => classSet({
    "px-4 py-2 font-medium cursor-pointer border-b-2 transition-colors": true,
    "border-primary-base text-primary-base": activeTab === tab,
    "border-transparent text-muted hover:text-foreground": activeTab !== tab,
  })

  return (
    <div className="p-4 rounded">
      <div className="flex border-b mb-4">
        <button className={tabClass("canned")} onClick={() => setActiveTab("canned")}>
          Canned Responses
        </button>
        <button className={tabClass("reasons")} onClick={() => setActiveTab("reasons")}>
          Report Reasons
        </button>
      </div>

      {activeTab === "canned" && (
        <>
          <div className="flex justify-between items-center mb-4">
            <HStack spacing={2}>
              <span className="font-medium whitespace-nowrap">Response Type:</span>
              <Select
                field="type"
                value={selectedType}
                options={typeOptions}
                onChange={handleTypeChange}
              />
            </HStack>
            <Button variant="primary" onClick={openCreateEditor}>
              <HStack spacing={2}>
                <Icon sprite={IconPlus} />
                <span><Trans id="action.new">New</Trans></span>
              </HStack>
            </Button>
          </div>

          {isLoading ? (
            <div className="py-8 text-center">
              <Loader />
            </div>
          ) : error ? (
            <div className="py-4 text-danger">{error}</div>
          ) : responses.length === 0 ? (
            <div className="py-8 text-center text-muted">
              <p><Trans id="responses.empty">No responses found. Click the "New" button to create one.</Trans></p>
            </div>
          ) : (
            <table className="w-full">
              <thead>
                <tr className="border-b">
                  <th className="text-left p-2"><Trans id="responses.table.title">Title</Trans></th>
                  <th className="text-left p-2"><Trans id="responses.table.content">Content</Trans></th>
                  <th className="text-left p-2"><Trans id="responses.table.duration">Duration</Trans></th>
                  <th className="text-center p-2"><Trans id="responses.table.actions">Actions</Trans></th>
                </tr>
              </thead>
              <tbody>
                {responses.map(response => (
                  <tr key={response.id} className="border-b">
                    <td className="p-2">{response.title}</td>
                    <td className="p-2">
                      <div className="truncate max-w-md">{response.content}</div>
                    </td>
                    <td className="p-2">{response.duration || "-"}</td>
                    <td className="p-2 text-center">
                      <Button 
                        variant="secondary" 
                        size="small" 
                        onClick={() => openEditEditor(response)}
                      >
                        <Icon sprite={IconEdit} className="h-4" />
                      </Button>
                      <Button 
                        variant="danger" 
                        size="small" 
                        className="ml-2"
                        onClick={() => confirmDelete(response.id)}
                      >
                        <Icon sprite={IconTrash} className="h-4" />
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </>
      )}

      {activeTab === "reasons" && (
        <>
          <div className="flex justify-between items-center mb-4">
            <span className="font-medium">Report Reasons</span>
            <Button variant="primary" onClick={openCreateReasonEditor}>
              <HStack spacing={2}>
                <Icon sprite={IconPlus} />
                <span><Trans id="action.new">New</Trans></span>
              </HStack>
            </Button>
          </div>

          {isLoadingReasons ? (
            <div className="py-8 text-center">
              <Loader />
            </div>
          ) : reasonsError ? (
            <div className="py-4 text-danger">{reasonsError}</div>
          ) : reasons.length === 0 ? (
            <div className="py-8 text-center text-muted">
              <p><Trans id="reportReasons.empty">No report reasons found. Click the "New" button to create one.</Trans></p>
            </div>
          ) : (
            <table className="w-full">
              <thead>
                <tr className="border-b">
                  <th className="w-10 p-2"></th>
                  <th className="text-left p-2"><Trans id="reportReasons.table.title">Title</Trans></th>
                  <th className="text-left p-2"><Trans id="reportReasons.table.description">Description</Trans></th>
                  <th className="text-center p-2"><Trans id="reportReasons.table.status">Status</Trans></th>
                  <th className="text-center p-2"><Trans id="reportReasons.table.actions">Actions</Trans></th>
                </tr>
              </thead>
              <tbody>
                {reasons.map((reason, index) => (
                  <tr
                    key={reason.id}
                    draggable
                    onDragStart={(e) => handleReasonDragStart(e, index)}
                    onDragOver={(e) => handleReasonDragOver(e, index)}
                    onDragLeave={handleReasonDragLeave}
                    onDrop={(e) => handleReasonDrop(e, index)}
                    onDragEnd={handleReasonDragEnd}
                    className={classSet({
                      "border-b transition-colors": true,
                      "opacity-50": !reason.isActive,
                      "opacity-40": draggedReasonIndex === index,
                      "border-t-2 border-t-primary-base": dragOverReasonIndex === index && draggedReasonIndex !== null && draggedReasonIndex > index,
                      "border-b-2 border-b-primary-base": dragOverReasonIndex === index && draggedReasonIndex !== null && draggedReasonIndex < index,
                    })}
                  >
                    <td className="p-2 cursor-grab active:cursor-grabbing">
                      <Icon sprite={IconMenu} className="h-4 w-4 text-subtle" />
                    </td>
                    <td className="p-2">{reason.title}</td>
                    <td className="p-2">
                      <div className="truncate max-w-md">{reason.description || "-"}</div>
                    </td>
                    <td className="p-2 text-center">
                      <span className={`px-2 py-1 rounded text-xs ${reason.isActive ? "bg-success-light text-success" : "bg-surface-alt text-muted"}`}>
                        {reason.isActive ? "Active" : "Inactive"}
                      </span>
                    </td>
                    <td className="p-2 text-center">
                      <Button variant="secondary" size="small" onClick={() => openEditReasonEditor(reason)}>
                        <Icon sprite={IconEdit} className="h-4" />
                      </Button>
                      <Button variant="danger" size="small" className="ml-2" onClick={() => confirmDeleteReason(reason.id)}>
                        <Icon sprite={IconTrash} className="h-4" />
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </>
      )}

      <Modal.Window 
        isOpen={editorState.isOpen} 
        onClose={closeEditor}
        center={false}
        size="large"
      >
        <Modal.Header>
          {editorState.id ? (
            <Trans id="responses.edit.title">Edit Response</Trans>
          ) : (
            <Trans id="responses.new.title">New Response</Trans>
          )}
        </Modal.Header>
        <Modal.Content>
          <Form error={editorState.error}>
            <Select
              field="type"
              label={i18n._("responses.type.label", { message: "Response Type" })}
              value={editorState.type}
              options={typeOptions}
              onChange={(opt) => opt && setEditorState(prev => ({ ...prev, type: opt.value }))}
            />
            <Input
              field="title"
              label={i18n._("responses.title.label", { message: "Title" })}
              value={editorState.title}
              onChange={(value) => setEditorState(prev => ({ ...prev, title: value }))}
              maxLength={100}
            />
            <TextArea
              field="content"
              label={i18n._("responses.content.label", { message: "Content" })}
              value={editorState.content}
              onChange={(value) => setEditorState(prev => ({ ...prev, content: value }))}
              minRows={5}
            />
            <Input
              field="duration"
              label={i18n._("responses.duration.label", { message: "Duration (e.g. 30m, 1h, 1d, optional for warnings)" })}
              value={editorState.duration}
              onChange={(value) => setEditorState(prev => ({ ...prev, duration: value }))}
              placeholder="e.g. 30m, 1h, 1d"
            />
            {editorState.id && (
              <Toggle
                field="isActive"
                label={i18n._("responses.active.label", { message: "Active" })}
                active={editorState.isActive}
                onToggle={() => setEditorState(prev => ({ ...prev, isActive: !prev.isActive }))}
              />
            )}
          </Form>
        </Modal.Content>
        <Modal.Footer>
          <Button variant="primary" onClick={handleEditorSubmit} disabled={isSubmitting}>
            {editorState.id ? (
              <Trans id="action.save">Save</Trans>
            ) : (
              <Trans id="action.create">Create</Trans>
            )}
          </Button>
          <Button variant="tertiary" onClick={closeEditor} disabled={isSubmitting}>
            <Trans id="action.cancel">Cancel</Trans>
          </Button>
        </Modal.Footer>
      </Modal.Window>

      <Modal.Window 
        isOpen={deleteConfirmation.isOpen} 
        onClose={() => setDeleteConfirmation({ isOpen: false })}
        center={true}
        size="small"
      >
        <Modal.Header>
          <Trans id="responses.delete.title">Delete Response</Trans>
        </Modal.Header>
        <Modal.Content>
          <p><Trans id="responses.delete.message">Are you sure you want to delete this response? This action cannot be undone.</Trans></p>
        </Modal.Content>
        <Modal.Footer>
          <Button variant="danger" onClick={handleDelete} disabled={isDeleting}>
            <Trans id="action.delete">Delete</Trans>
          </Button>
          <Button variant="tertiary" onClick={() => setDeleteConfirmation({ isOpen: false })} disabled={isDeleting}>
            <Trans id="action.cancel">Cancel</Trans>
          </Button>
        </Modal.Footer>
      </Modal.Window>

      <Modal.Window isOpen={reasonEditorState.isOpen} onClose={closeReasonEditor} center={false} size="large">
        <Modal.Header>
          {reasonEditorState.id ? (
            <Trans id="reportReasons.edit.title">Edit Report Reason</Trans>
          ) : (
            <Trans id="reportReasons.new.title">New Report Reason</Trans>
          )}
        </Modal.Header>
        <Modal.Content>
          <Form error={reasonEditorState.error}>
            <Input
              field="title"
              label={i18n._("reportReasons.title.label", { message: "Title" })}
              value={reasonEditorState.title}
              onChange={(value) => setReasonEditorState((prev) => ({ ...prev, title: value }))}
              maxLength={100}
            />
            <TextArea
              field="description"
              label={i18n._("reportReasons.description.label", { message: "Description (optional)" })}
              value={reasonEditorState.description}
              onChange={(value) => setReasonEditorState((prev) => ({ ...prev, description: value }))}
              minRows={3}
            />
            {reasonEditorState.id && (
              <Toggle
                field="isActive"
                label={i18n._("reportReasons.active.label", { message: "Active" })}
                active={reasonEditorState.isActive}
                onToggle={() => setReasonEditorState((prev) => ({ ...prev, isActive: !prev.isActive }))}
              />
            )}
          </Form>
        </Modal.Content>
        <Modal.Footer>
          <Button variant="primary" onClick={handleReasonEditorSubmit} disabled={isSubmitting}>
            {reasonEditorState.id ? (
              <Trans id="action.save">Save</Trans>
            ) : (
              <Trans id="action.create">Create</Trans>
            )}
          </Button>
          <Button variant="tertiary" onClick={closeReasonEditor} disabled={isSubmitting}>
            <Trans id="action.cancel">Cancel</Trans>
          </Button>
        </Modal.Footer>
      </Modal.Window>

      <Modal.Window
        isOpen={reasonDeleteConfirmation.isOpen}
        onClose={() => setReasonDeleteConfirmation({ isOpen: false })}
        center={true}
        size="small"
      >
        <Modal.Header>
          <Trans id="reportReasons.delete.title">Delete Report Reason</Trans>
        </Modal.Header>
        <Modal.Content>
          <p>
            <Trans id="reportReasons.delete.message">
              Are you sure you want to delete this report reason? This action cannot be undone.
            </Trans>
          </p>
        </Modal.Content>
        <Modal.Footer>
          <Button variant="danger" onClick={handleDeleteReason} disabled={isDeleting}>
            <Trans id="action.delete">Delete</Trans>
          </Button>
          <Button variant="tertiary" onClick={() => setReasonDeleteConfirmation({ isOpen: false })} disabled={isDeleting}>
            <Trans id="action.cancel">Cancel</Trans>
          </Button>
        </Modal.Footer>
      </Modal.Window>
    </div>
  )
}

export default ManageCannedResponsesPage
