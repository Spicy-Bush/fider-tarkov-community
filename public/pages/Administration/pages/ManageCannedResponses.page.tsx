import React, { useState, useEffect } from "react"
import { 
  Button, 
  Form, 
  Loader, 
  Input, 
  TextArea, 
  Select, 
  SelectOption, 
  Modal, 
  Icon 
} from "@fider/components"
import { HStack } from "@fider/components/layout"
import { actions, Failure, notify } from "@fider/services"
import { AdminBasePage } from "../components/AdminBasePage"
import IconPlus from "@fider/assets/images/heroicons-plus-circle.svg"
import IconEdit from "@fider/assets/images/heroicons-pencil-alt.svg"
import IconTrash from "@fider/assets/images/heroicons-trash.svg"
import { CannedResponse } from "@fider/services/actions/response"
import { Trans } from "@lingui/react/macro"
import { i18n } from "@lingui/core"

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

const ManageCannedResponsesContent: React.FC<{ types?: string[] | null }> = (props) => {
  const safeTypes = React.useMemo(() => {
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
    type: selectedType,
    title: "",
    content: "",
    duration: "",
    isActive: true
  })
  const [deleteConfirmation, setDeleteConfirmation] = useState<{ isOpen: boolean; id?: number }>({
    isOpen: false
  })

  useEffect(() => {
    loadResponses()
  }, [selectedType])

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

    let result: { ok: boolean; error?: Failure }
    if (id) {
      // Update
      result = await actions.updateCannedResponse(id, { 
        type, title, content, duration, isActive 
      })
    } else {
      // Create
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
  }

  const confirmDelete = (id: number) => {
    setDeleteConfirmation({ isOpen: true, id })
  }

  const handleDelete = async () => {
    const { id } = deleteConfirmation
    if (id) {
      const result = await actions.deleteCannedResponse(id)
      if (result.ok) {
        notify.success(i18n._("responses.deleted.success", { message: "Response deleted successfully" }))
        setDeleteConfirmation({ isOpen: false })
        loadResponses()
      } else {
        notify.error(i18n._("responses.deleted.error", { message: "Failed to delete response" }))
      }
    }
  }

  const typeOptions = safeTypes.map(type => ({
    value: type,
    label: type.charAt(0).toUpperCase() + type.slice(1) // Capitalize first letter
  }))

  return (
    <div className="p-4 rounded">
      <div className="flex justify-between items-center mb-4">
        <div className="w-64">
          <Select
            key={`type-select-${selectedType}`}
            field="type"
            label={i18n._("responses.type.label", { message: "Response Type" })}
            defaultValue={selectedType}
            options={typeOptions}
            onChange={handleTypeChange}
          />
        </div>
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
        <div className="py-4 text-red-500">{error}</div>
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

      {/* Editor Modal */}
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
              key={`editor-type-${editorState.isOpen}-${editorState.type}`}
              field="type"
              label={i18n._("responses.type.label", { message: "Response Type" })}
              defaultValue={editorState.type}
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
              <div className="mt-4">
                <label className="flex items-center cursor-pointer">
                  <input
                    type="checkbox"
                    checked={editorState.isActive}
                    onChange={(e) => setEditorState(prev => ({ ...prev, isActive: e.target.checked }))}
                    className="mr-2"
                  />
                  <span><Trans id="responses.active.label">Active</Trans></span>
                </label>
              </div>
            )}
          </Form>
        </Modal.Content>
        <Modal.Footer>
          <Button variant="primary" onClick={handleEditorSubmit}>
            {editorState.id ? (
              <Trans id="action.save">Save</Trans>
            ) : (
              <Trans id="action.create">Create</Trans>
            )}
          </Button>
          <Button variant="tertiary" onClick={closeEditor}>
            <Trans id="action.cancel">Cancel</Trans>
          </Button>
        </Modal.Footer>
      </Modal.Window>

      {/* Delete Confirmation Modal */}
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
          <Button variant="danger" onClick={handleDelete}>
            <Trans id="action.delete">Delete</Trans>
          </Button>
          <Button variant="tertiary" onClick={() => setDeleteConfirmation({ isOpen: false })}>
            <Trans id="action.cancel">Cancel</Trans>
          </Button>
        </Modal.Footer>
      </Modal.Window>
    </div>
  )
}

export default class ManageCannedResponsesPage extends AdminBasePage<ManageCannedResponsesPageProps, {}> {
  public id = "p-admin-responses"
  public name = "responses"
  public title = "Canned Responses"
  public subtitle = "Manage predefined responses for user moderation"

  public content() {
    return <ManageCannedResponsesContent types={this.props.types} />
  }
} 