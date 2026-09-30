import React, { useEffect, useId, useRef, useState } from "react"
import { Button, DisplayError, ImageUploader, Input, Modal } from "@fider/components"
import { ImageUpload } from "@fider/models"
import { Failure } from "@fider/services"
import {
  deleteFiles, FileInfo, fileRequestFailure, FileUploadRequest, FileUsageResponse,
  getFileUsage, pruneFiles, PruneFilesRequest, renameFile, uploadFile, newFileUploadID, FileRemoval,
} from "@fider/services/actions/file"
import { FileCheckbox } from "./FileControls"

export const FileError = ({ error }: { error?: Failure }) => (
  <DisplayError error={error} fields={Array.from(new Set(error?.errors?.map(item => item.field || "")))} />
)

export const FileDialog = (props: {
  title: string
  isOpen?: boolean
  busy?: boolean
  large?: boolean
  children: React.ReactNode
  footer: React.ReactNode
  onClose: () => void
}) => {
  const titleID = useId()
  return (
    <Modal.Window
      isOpen={props.isOpen ?? true}
      onClose={props.onClose}
      canClose={!props.busy}
      labelledBy={titleID}
      manageHistory={false}
      size={props.large ? "large" : "small"}
    >
      <Modal.Header>
        <h2 id={titleID}>{props.title}</h2>
      </Modal.Header>
      <Modal.Content>{props.children}</Modal.Content>
      <Modal.Footer>
        <div className="flex flex-wrap justify-end gap-2">{props.footer}</div>
      </Modal.Footer>
    </Modal.Window>
  )
}

export const UploadFileDialog = (props: { isOpen: boolean; onClose: () => void; onUploaded: () => void }) => {
  const [name, setName] = useState("")
  const [type, setType] = useState<"file" | "attachment">("file")
  const [image, setImage] = useState<ImageUpload>()
  const [busy, setBusy] = useState(false)
  const [unconfirmed, setUnconfirmed] = useState(false)
  const [error, setError] = useState<Failure>()
  const submission = useRef<FileUploadRequest>()
  const reading = useRef<Promise<ImageUpload | undefined>>()

  const changed = () => {
    submission.current = undefined
    setError(undefined)
  }

  const upload = async () => {
    setBusy(true)
    setError(undefined)
    try {
      const file = reading.current ? await reading.current : image
      if (!file?.upload) {
        setError({ errors: [{ message: "Select an image before uploading." }] })
        return
      }
      const uploadName = name.trim() || file.upload.fileName
      if (!uploadName) {
        setError({ errors: [{ message: "Enter a name for this image." }] })
        return
      }
      if (!submission.current) {
        const identity = await newFileUploadID()
        if (!identity.ok) {
          setError(identity.error)
          return
        }

        submission.current = {
          submissionId: identity.data,
          name: uploadName,
          file,
          uploadType: type,
        }
      }
      const result = await uploadFile(submission.current)
      if (!result.ok) {
        setError(result.error)
        setUnconfirmed(unconfirmed || result.unconfirmed)
        return
      }
      setName("")
      setImage(undefined)
      setType("file")
      setUnconfirmed(false)
      submission.current = undefined
      reading.current = undefined
      props.onUploaded()
      props.onClose()
    } catch (cause) {
      const failure = fileRequestFailure(cause, "The upload could not be confirmed. Retry to check the same upload.")
      setUnconfirmed(!!submission.current)
      setError(failure)
    } finally {
      setBusy(false)
    }
  }

  return (
    <FileDialog
      title="Upload image"
      isOpen={props.isOpen}
      busy={busy}
      onClose={props.onClose}
      footer={<>
        <Button onClick={props.onClose} disabled={busy}>Cancel</Button>
        <Button variant="primary" onClick={upload} loading={busy}>
          {unconfirmed ? "Retry upload" : "Upload image"}
        </Button>
      </>}
    >
      <FileError error={error} />
      {unconfirmed && <Button variant="tertiary" onClick={() => {
        changed()
        setUnconfirmed(false)
      }}>Edit as a new upload</Button>}
      <Input field="file-name" label="Name" placeholder="Use the image filename" value={name} disabled={busy || unconfirmed}
        onChange={value => {
          setName(value)
          changed()
        }} />
      <label className="block mb-4 text-sm font-medium">
        Visibility
        <select value={type} disabled={busy || unconfirmed} className="block w-full mt-1 rounded-input border border-border bg-elevated p-2"
          onChange={event => {
            setType(event.target.value as "file" | "attachment")
            changed()
          }}>
          <option value="file">File managers only</option>
          <option value="attachment">Public image</option>
        </select>
      </label>
      <ImageUploader field="file-image" label="Image" initialUpload={image} disabled={busy || unconfirmed}
        onRead={promise => {
          reading.current = promise
          changed()
        }}
        onChange={value => {
          setImage(value)
          if (value.remove) {
            reading.current = undefined
            changed()
          }
        }} />
    </FileDialog>
  )
}

export const RenameFileDialog = (props: { file: FileInfo; onClose: () => void; onRenamed: () => void }) => {
  const [name, setName] = useState(props.file.name)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<Failure>()

  const save = async () => {
    setBusy(true)
    setError(undefined)
    try {
      const result = await renameFile(props.file.blobKey, name.trim())
      if (result.ok) {
        props.onRenamed()
        props.onClose()
      } else {
        setError(result.error)
      }
    } catch (cause) {
      setError(fileRequestFailure(cause, "The name could not be saved. Try again."))
    } finally {
      setBusy(false)
    }
  }

  return (
    <FileDialog
      title="Rename image"
      busy={busy}
      onClose={props.onClose}
      footer={<>
        <Button onClick={props.onClose} disabled={busy}>Cancel</Button>
        <Button variant="primary" onClick={save} loading={busy} disabled={!name.trim()}>Save name</Button>
      </>}
    >
      <form onSubmit={event => {
        event.preventDefault()
        if (name.trim() && !busy) void save()
      }}>
        <FileError error={error} />
        <Input field="rename-file" label="Name" value={name} disabled={busy} onChange={setName} />
      </form>
    </FileDialog>
  )
}

export const FileUsageDialog = (props: { files: FileInfo[]; onClose: () => void }) => {
  const [location, setLocation] = useState({ index: 0, page: 1 })
  const [revision, setRevision] = useState(0)
  const [usage, setUsage] = useState<FileUsageResponse>()
  const [error, setError] = useState<Failure>()
  const file = props.files[location.index]

  useEffect(() => {
    const controller = new AbortController()
    setUsage(undefined)
    setError(undefined)
    void getFileUsage(file.blobKey, location.page, controller.signal).then(result => {
      if (controller.signal.aborted) return
      if (result.ok) {
        setUsage(result.data)
      } else {
        setError(result.error)
      }
    }).catch(cause => {
      if (!controller.signal.aborted) setError(fileRequestFailure(cause, "Usage could not be loaded. Try again."))
    })
    return () => controller.abort()
  }, [file.blobKey, location.page, revision])

  return (
    <FileDialog title="Image usage" large onClose={props.onClose} footer={<Button onClick={props.onClose}>Close</Button>}>
      {props.files.length > 1 && (
        <label className="block mb-4 text-sm">
          Selected image
          <select className="block w-full mt-1 rounded-input border border-border bg-elevated p-2" value={location.index}
            onChange={event => setLocation({ index: Number(event.target.value), page: 1 })}>
            {props.files.map((item, index) => <option key={item.blobKey} value={index}>{item.name}</option>)}
          </select>
        </label>
      )}
      <p className="font-medium break-words mb-3">{file.name}</p>
      {error ? <><FileError error={error} /><Button onClick={() => setRevision(value => value + 1)}>Retry usage</Button></> : !usage ? (
        <p role="status">Loading usage</p>
      ) : (
        <>
          {usage.total === 0 ? <p>This image is unused.</p> : <p className="text-sm text-muted mb-3">{usage.total} references</p>}
          <ul className="divide-y divide-border text-sm">
            {usage.items.map((item, index) => (
              <li key={`${item.kind}:${item.id}:${index}`} className="py-3 break-words">
                <span className="text-muted mr-2">{item.kind}</span>
                {item.url ? <a href={item.url} className="text-primary underline" target="_blank" rel="noopener noreferrer">{item.title}</a> : item.title}
                {item.scope !== "active" && <span className="text-muted ml-2">({item.scope === "deleted" ? "deleted content" : "draft"})</span>}
              </li>
            ))}
          </ul>
          {usage.totalPages > 1 && <div className="flex items-center gap-3 mt-4">
            <Button disabled={usage.page === 1} onClick={() => setLocation(previous => ({ ...previous, page: usage.page - 1 }))}>Previous references</Button>
            <span>{usage.page} / {usage.totalPages}</span>
            <Button disabled={usage.page === usage.totalPages} onClick={() => setLocation(previous => ({ ...previous, page: usage.page + 1 }))}>Next references</Button>
          </div>}
        </>
      )}
    </FileDialog>
  )
}

export const DeleteFilesDialog = (props: {
  files: FileInfo[]
  scope: Omit<FileRemoval, "force">
  onClose: () => void
  onChanged: (deleted: string[], pending: string[]) => void
}) => {
  const [remaining, setRemaining] = useState(props.files)
  const [force, setForce] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<Failure>()
  const [progress, setProgress] = useState({ deleted: 0, pending: 0 })

  const remove = async () => {
    setBusy(true)
    setError(undefined)
    let unfinished = remaining
    const issues: Array<{ message: string }> = []

    try {
      for (let offset = 0; offset < remaining.length; offset += 100) {
        const batch = remaining.slice(offset, offset + 100)
        const result = await deleteFiles(batch.map(file => file.blobKey), { ...props.scope, force })
        if (!result.ok) {
          setError({ errors: [...issues, ...(result.error.errors || [])] })
          return
        }

        const accepted = new Set([...result.data.deleted, ...result.data.pending])
        const skipped = new Set(result.data.skipped)
        unfinished = unfinished.filter(file => !accepted.has(file.blobKey)).map(file => (
          skipped.has(file.blobKey) ? { ...file, hasProtectedReferences: true } : file
        ))
        setRemaining(unfinished)
        setProgress(previous => ({
          deleted: previous.deleted + result.data.deleted.length,
          pending: previous.pending + result.data.pending.length,
        }))

        for (const item of result.data.errors) {
          const file = batch.find(file => file.blobKey === item.blobKey)!
          issues.push({ message: `${file.name}: ${item.message}` })
        }
        for (const key of result.data.skipped) {
          const file = batch.find(file => file.blobKey === key)!
          issues.push({ message: `${file.name} is in use and was kept.` })
        }

        if (accepted.size > 0) {
          props.onChanged(result.data.deleted, result.data.pending)
        }
      }

      if (unfinished.length === 0) {
        props.onClose()
      } else {
        setError({ errors: issues })
      }
    } catch (cause) {
      const failure = fileRequestFailure(cause, "Deletion could not be confirmed. Retry to check the remaining files.")
      setError({ errors: [...issues, ...(failure.errors || [])] })
    } finally {
      setBusy(false)
    }
  }

  return (
    <FileDialog
      title={`Delete ${remaining.length} ${remaining.length === 1 ? "image" : "images"}`}
      busy={busy}
      onClose={props.onClose}
      footer={<>
        <Button disabled={busy} onClick={props.onClose}>Cancel</Button>
        <Button variant="danger" loading={busy} onClick={remove}>Delete selected</Button>
      </>}
    >
      <FileError error={error} />
      {(progress.deleted > 0 || progress.pending > 0) && (
        <p role="status" className="mb-3 text-sm">{progress.deleted} deleted, {progress.pending} queued, {remaining.length} remaining</p>
      )}
      <ul className="max-h-64 overflow-y-auto text-sm mb-4">
        {remaining.map(file => <li className="py-1 break-words" key={file.blobKey}>{file.name}</li>)}
      </ul>
      <FileCheckbox checked={force} disabled={busy} onChange={setForce}>
        Also delete images with protected references.
      </FileCheckbox>
    </FileDialog>
  )
}

export const PruneFilesDialog = (props: { request: PruneFilesRequest; count: number; onClose: () => void; onChanged: () => void }) => {
  const [progress, setProgress] = useState({ deleted: 0, pending: 0, skipped: 0, errors: 0 })
  const [issues, setIssues] = useState<string[]>([])
  const [phase, setPhase] = useState<"review" | "running" | "paused" | "failed" | "done">("review")
  const [error, setError] = useState<Failure>()
  const cursor = useRef<string>()
  const stopped = useRef(false)

  useEffect(() => () => { stopped.current = true }, [])

  const prune = async () => {
    stopped.current = false
    setPhase("running")
    setError(undefined)
    try {
      do {
        const result = await pruneFiles({ ...props.request, cursor: cursor.current })
        if (!result.ok) {
          setError(result.error)
          setPhase("failed")
          return
        }
        const batch = result.data
        setProgress(previous => ({
          deleted: previous.deleted + batch.deleted.length,
          pending: previous.pending + batch.pending.length,
          skipped: previous.skipped + batch.skipped.length,
          errors: previous.errors + batch.errors.length,
        }))
        setIssues(previous => [...previous, ...batch.errors.map(item => `${item.blobKey}: ${item.message}`)].slice(0, 20))
        cursor.current = batch.nextCursor
        props.onChanged()
        if (!batch.nextCursor) {
          setPhase("done")
          return
        }
      } while (!stopped.current)
      setPhase("paused")
    } catch (cause) {
      setError(fileRequestFailure(cause, "Cleanup could not be confirmed. Retry to continue the same cleanup."))
      setPhase("failed")
    }
  }

  return (
    <FileDialog title="Delete matching unused images" busy={phase === "running"} onClose={props.onClose}
      footer={phase === "running" ? <Button onClick={() => { stopped.current = true }}>Stop after this batch</Button> : <>
        <Button onClick={props.onClose}>{phase === "done" ? "Close" : "Cancel"}</Button>
        {phase !== "done" && <Button variant="danger" onClick={prune}>{phase === "review" ? "Delete matching unused images" : "Continue cleanup"}</Button>}
      </>}>
      <p className="mb-3">Delete {props.count} matching images?</p>
      <ul className="text-sm mb-3 space-y-1">
        <li>Deleted content references: {props.request.includeDeleted ? "included in cleanup" : "protected"}</li>
        <li>Draft references: {props.request.includeDrafts ? "included in cleanup" : "protected"}</li>
      </ul>
      <p className="text-sm text-muted mb-4">Images uploaded after {new Date(props.request.before).toLocaleString()} are excluded.</p>
      <FileError error={error} />
      {phase !== "review" && <p role="status" className="mb-3">
        {progress.deleted} deleted, {progress.pending} queued, {progress.skipped} kept, {progress.errors} failed
        {phase === "running" ? ", cleaning" : phase === "paused" ? ", stopped" : phase === "done" ? ", complete" : ""}
      </p>}
      {progress.pending > 0 && <p className="text-sm mb-3">Queued deletions will continue in the background.</p>}
      {issues.length > 0 && <ul className="text-sm text-danger max-h-56 overflow-y-auto">
        {issues.map((issue, index) => <li key={index} className="mb-2 break-words">{issue}</li>)}
      </ul>}
    </FileDialog>
  )
}
