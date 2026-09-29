import React, { useEffect, useState } from "react"
import { Button } from "@fider/components"
import { PageConfig } from "@fider/components/layouts"
import { copyToClipboard, Failure, notify } from "@fider/services"
import {
  FileInfo, FileLibraryOptions, FileListResponse, FileQuery, FileType, FileUsage,
  fileDownloadURL, filePreviewURL, fileRequestFailure, listFiles, PruneFilesRequest, refreshFileInventory,
} from "@fider/services/actions/file"
import {
  DeleteFilesDialog, FileDialog, FileError, FileUsageDialog, PruneFilesDialog, RenameFileDialog, UploadFileDialog,
} from "./FileManagement/FileDialogs"
import { FileCheckbox, FilePagination, FileSelect } from "./FileManagement/FileControls"

export const pageConfig: PageConfig = {
  title: "Media library",
  subtitle: "Find, reuse and clean up your images",
  sidebarItem: "files",
}

const readQuery = (options: FileLibraryOptions): FileQuery => {
  const defaults = options.defaults
  if (typeof window === "undefined") return defaults
  const params = new URLSearchParams(window.location.search)
  const page = Number(params.get("page"))
  const pageSize = Number(params.get("pageSize"))
  const type = params.get("type") as FileType
  const usage = params.get("usage") as FileUsage
  const sortBy = params.get("sortBy") as FileQuery["sortBy"]
  const sortDir = params.get("sortDir") as FileQuery["sortDir"]
  return {
    page: Number.isInteger(page) && page > 0 && page <= options.maxPage ? page : defaults.page,
    pageSize: options.pageSizes.includes(pageSize) ? pageSize : defaults.pageSize,
    search: params.get("search") || "",
    type: options.types.some(item => item.value === type) ? type : defaults.type,
    usage: options.usage.some(item => item.value === usage) ? usage : defaults.usage,
    includeDeleted: params.has("includeDeleted") ? params.get("includeDeleted") === "true" : defaults.includeDeleted,
    includeDrafts: params.has("includeDrafts") ? params.get("includeDrafts") === "true" : defaults.includeDrafts,
    sortBy: options.sort.some(item => item.value === sortBy) ? sortBy : defaults.sortBy,
    sortDir: sortDir === "asc" || sortDir === "desc" ? sortDir : defaults.sortDir,
  }
}

const formatSize = (bytes: number) => {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

type Listing = { query: FileQuery } & (
  | { status: "loading" }
  | { status: "error"; error: Failure }
  | { status: "ready"; data: FileListResponse }
)

type Dialog =
  | { kind: "upload" }
  | { kind: "rename"; file: FileInfo }
  | { kind: "preview"; files: FileInfo[]; index: number }
  | { kind: "selection" }
  | { kind: "usage"; files: FileInfo[] }
  | { kind: "delete"; files: FileInfo[]; scope: Pick<FileQuery, "includeDeleted" | "includeDrafts"> }
  | { kind: "prune"; request: PruneFilesRequest; count: number }

const controlClass = "min-w-0 rounded-input border border-border bg-elevated px-3 py-2 text-sm text-foreground"
const actionClass = "rounded-button px-2 py-1.5 text-xs text-muted hover:bg-surface-alt hover:text-foreground focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary"
const toggleClass = "cursor-pointer rounded-button border border-border px-2.5 py-1.5 text-xs font-medium hover:bg-surface-alt aria-pressed:border-primary aria-pressed:bg-primary aria-pressed:text-white aria-[pressed=mixed]:border-primary disabled:cursor-not-allowed disabled:opacity-50 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary"
const FileThumbnail = ({ file, grid, revision, maxBytes }: { file: FileInfo; grid: boolean; revision: number; maxBytes: number }) => {
  const [failed, setFailed] = useState(false)

  useEffect(() => setFailed(false), [file.thumbnailURL, revision])

  return failed || file.size > maxBytes ? (
    <span className={`flex items-center justify-center bg-surface-alt text-xs text-muted ${grid ? "h-40 w-full" : "h-16 w-16"}`}>No preview</span>
  ) : (
    <img src={file.thumbnailURL} alt="" loading="lazy" decoding="async" width={200} height={200}
      className={`rounded-input object-contain bg-surface-alt ${grid ? "h-40 w-full" : "h-16 w-16"}`}
      onError={() => setFailed(true)} />
  )
}

const FileManagementPage = ({ options }: { options: FileLibraryOptions }) => {
  const maxPreviewBytes = options.maxImageBytes
  const [query, setQuery] = useState(() => readQuery(options))
  const [search, setSearch] = useState(query.search)
  const [listing, setListing] = useState<Listing>({ query, status: "loading" })
  const [refreshError, setRefreshError] = useState<Failure>()
  const [revision, setRevision] = useState(0)
  const [selection, setSelection] = useState(new Map<string, FileInfo>())
  const [grid, setGrid] = useState(false)
  const [dialog, setDialog] = useState<Dialog>()

  useEffect(() => {
    if (search === query.search) return
    const timer = setTimeout(() => setQuery(previous => ({ ...previous, search, page: 1 })), 200)
    return () => clearTimeout(timer)
  }, [search, query.search])

  useEffect(() => {
    const controller = new AbortController()
    const params = new URLSearchParams({
      ...query,
      page: String(query.page),
      pageSize: String(query.pageSize),
      includeDeleted: String(query.includeDeleted),
      includeDrafts: String(query.includeDrafts),
    })
    history.replaceState(history.state, "", `${location.pathname}?${params}`)

    void listFiles(query, controller.signal).then(result => {
      if (controller.signal.aborted) return
      if (result.ok) {
        setListing({ query, status: "ready", data: result.data })
        setSelection(previous => {
          if (previous.size === 0) return previous

          const next = new Map(previous)
          for (const file of result.data.files) {
            if (next.has(file.blobKey)) next.set(file.blobKey, file)
          }
          return next
        })
      } else {
        setListing({ query, status: "error", error: result.error })
      }
    }).catch(cause => {
      if (!controller.signal.aborted) {
        setListing({ query, status: "error", error: fileRequestFailure(cause, "Images could not be loaded. Check your connection and try again.") })
      }
    })

    return () => controller.abort()
  }, [query, revision])

  useEffect(() => {
    if (listing.query !== query || listing.status !== "ready") return
    const data = listing.data
    if (data.inventory.state === "ready" && !data.files.some(file => file.state === "deleting")) return
    const timer = setTimeout(() => setRevision(value => value + 1), 5000)
    return () => clearTimeout(timer)
  }, [listing, query])

  const loading = listing.query !== query || listing.status === "loading"
  const data = listing.query === query && listing.status === "ready" ? listing.data : undefined
  const files = data?.files || []
  const selectable = files.filter(file => file.state === "ready")
  const selectedFiles = Array.from(selection.values())
  const selectedOnPage = selectable.filter(file => selection.has(file.blobKey)).length
  const readySelection = selectedFiles.filter(file => file.state === "ready")
  const downloadableFiles = readySelection.filter(file => file.size <= maxPreviewBytes)
  const preview = dialog?.kind === "preview" ? dialog.files[dialog.index] : undefined

  const refresh = () => setRevision(value => value + 1)
  const rescan = async () => {
    setRefreshError(undefined)
    try {
      const result = await refreshFileInventory()
      if (result.ok) refresh()
      else setRefreshError(result.error)
    } catch (cause) {
      setRefreshError(fileRequestFailure(cause, "Image discovery could not be started. Try Refresh again."))
    }
  }
  const closeDialog = () => setDialog(undefined)
  const changeQuery = (change: Partial<FileQuery>) => setQuery(previous => ({ ...previous, ...change, search, page: 1 }))

  const clearFilters = () => {
    setSearch("")
    setQuery(previous => ({
      ...previous,
      search: "",
      type: options.defaults.type,
      usage: options.defaults.usage,
      includeDeleted: options.defaults.includeDeleted,
      includeDrafts: options.defaults.includeDrafts,
      page: options.defaults.page,
    }))
  }

  const toggleFile = (file: FileInfo) => {
    setSelection(previous => {
      const next = new Map(previous)
      if (next.has(file.blobKey)) {
        next.delete(file.blobKey)
      } else {
        next.set(file.blobKey, file)
      }
      return next
    })
  }

  const togglePage = (checked: boolean) => {
    setSelection(previous => {
      const next = new Map(previous)
      for (const file of selectable) {
        if (checked) {
          next.set(file.blobKey, file)
        } else {
          next.delete(file.blobKey)
        }
      }
      return next
    })
  }

  const deleted = (deletedKeys: string[], pendingKeys: string[]) => {
    setSelection(previous => {
      const next = new Map(previous)
      for (const key of deletedKeys) next.delete(key)
      for (const key of pendingKeys) {
        const file = next.get(key)
        if (file) next.set(key, { ...file, state: "deleting" })
      }
      return next
    })
    refresh()
  }

  const changePage = (page: number) => {
    setQuery(previous => ({ ...previous, page }))
    document.getElementById("file-results")?.scrollIntoView({ block: "start" })
  }

  const copy = async (file: FileInfo) => {
    try {
      await copyToClipboard(new URL(file.url, location.origin).href)
      notify.success("Image link copied")
    } catch {
      notify.error("The browser could not copy the link. Open the image to copy its address.")
    }
  }

  const downloadSelected = () => {
    for (const file of downloadableFiles) {
      const link = document.createElement("a")
      link.href = fileDownloadURL(file.blobKey)
      link.download = file.name
      document.body.appendChild(link)
      link.click()
      link.remove()
    }
  }

  const hasFilters = !!query.search || query.type !== "all" || query.usage !== "all" || query.includeDeleted || query.includeDrafts
  const deletionScope = { includeDeleted: query.includeDeleted, includeDrafts: query.includeDrafts }

  return (
    <div className="min-w-0 space-y-4 pb-36 max-md:pb-60">
      <header>
        <h1 className="text-xl font-semibold">Media library</h1>
      </header>
      <div className="flex flex-wrap items-end gap-3">
        <label className="flex-1 min-w-[160px] text-sm font-medium">
          Search images
          <input type="search" value={search} onChange={event => setSearch(event.target.value)}
            placeholder="Search by name" className={`${controlClass} block w-full mt-1`} />
        </label>
        <Button onClick={() => changeQuery({ usage: "unused" })}>Review unused</Button>
        <Button variant="primary" loading={false} onClick={() => setDialog({ kind: "upload" })}>Upload image</Button>
      </div>

      <div className="flex flex-wrap items-end gap-3 rounded-card border border-border bg-elevated p-3">
        <FileSelect label="Type" value={query.type} onChange={type => changeQuery({ type: type as FileType })} className="min-w-[160px] flex-1">
          {options.types.map(type => <option key={type.value} value={type.value}>{type.label}</option>)}
        </FileSelect>
        <FileSelect label="Usage" value={query.usage} onChange={usage => changeQuery({ usage: usage as FileUsage })} className="min-w-[120px]">
          {options.usage.map(usage => <option key={usage.value} value={usage.value}>{usage.label}</option>)}
        </FileSelect>
        <FileSelect label="Sort by" value={query.sortBy} onChange={sortBy => changeQuery({ sortBy: sortBy as FileQuery["sortBy"] })} className="min-w-[140px]">
          {options.sort.map(sort => <option key={sort.value} value={sort.value}>{sort.label}</option>)}
        </FileSelect>
        <div role="group" aria-label="Order" className="flex gap-1">
          <button type="button" className={toggleClass}
            aria-pressed={query.sortDir === "asc"} onClick={() => changeQuery({ sortDir: "asc" })}>
            Ascending
          </button>
          <button type="button" className={toggleClass}
            aria-pressed={query.sortDir === "desc"} onClick={() => changeQuery({ sortDir: "desc" })}>
            Descending
          </button>
        </div>
        <Button onClick={rescan}>Refresh</Button>
        {hasFilters && <Button variant="tertiary" onClick={clearFilters}>Clear filters</Button>}
      </div>

      <FileError error={refreshError} />

      <fieldset className="rounded-card border border-border p-3 text-sm">
          <legend className="px-1 font-medium">Cleanup scope</legend>
          <div className="flex flex-wrap gap-x-5 gap-y-3">
            <FileCheckbox checked={query.includeDeleted} onChange={includeDeleted => changeQuery({ includeDeleted })}>
              Allow cleanup of images in deleted content
            </FileCheckbox>
            <FileCheckbox checked={query.includeDrafts} onChange={includeDrafts => changeQuery({ includeDrafts })}>
              Allow cleanup of images in drafts
            </FileCheckbox>
          </div>
      </fieldset>

      <div id="file-results" className="flex scroll-mt-4 flex-wrap items-end justify-between gap-3 text-sm">
        <div role="group" aria-label="Display">
          <button type="button" className={actionClass} aria-pressed={!grid} onClick={() => setGrid(false)}>List</button>
          <button type="button" className={actionClass} aria-pressed={grid} onClick={() => setGrid(true)}>Grid</button>
        </div>
        {data && <FilePagination page={data.page} totalPages={data.totalPages} onChange={changePage} />}
        <div className="flex items-end gap-3">
          <button type="button" className={toggleClass} disabled={selectable.length === 0}
            aria-pressed={selectedOnPage > 0 && (selectedOnPage === selectable.length ? true : "mixed")}
            onClick={() => togglePage(selectedOnPage !== selectable.length)}>
            Select this page
          </button>
          <FileSelect label="Per page" value={query.pageSize} onChange={value => changeQuery({ pageSize: Number(value) })}>
            {options.pageSizes.map(size => <option key={size} value={size}>{size}</option>)}
          </FileSelect>
        </div>
      </div>

      {loading && <p role="status" className="py-8 text-center text-muted">Loading images</p>}
      {listing.query === query && listing.status === "error" && <div className="rounded-card border border-danger p-4">
        <FileError error={listing.error} /><Button onClick={refresh}>Retry loading images</Button>
      </div>}

      {data && <>
        {data.inventory.state !== "ready" && <p role="status" className="rounded-card bg-surface-alt p-3 text-sm">
          {data.inventory.state === "retrying" ? "Image discovery will retry automatically." : "Discovering images"}
          {` ${data.inventory.scanned} checked.`}
        </p>}
        {data.inventory.lastError && <FileError error={{ errors: [{ message: data.inventory.lastError }] }} />}
        {data.inventory.skipped > 0 && <p role="status" className="text-sm text-danger">
          {data.inventory.skipped} stored {data.inventory.skipped === 1 ? "file has an invalid name and was" : "files have invalid names and were"} skipped.
        </p>}
        {query.usage === "unused" && data.total > 0 && <Button size="small" loading={false} disabled={data.inventory.state !== "ready"} onClick={() => setDialog({
            kind: "prune",
            count: data.total,
            request: {
              search: query.search,
              type: query.type,
              before: data.listedAt,
              includeDeleted: query.includeDeleted,
              includeDrafts: query.includeDrafts,
            },
          })}>Review all {data.total} matching unused images</Button>}

        {files.length === 0 ? data.inventory.state === "ready" && <div className="py-10 text-center">
          <p className="font-medium">No images found</p>
          {hasFilters && <Button variant="tertiary" onClick={clearFilters}>Clear filters</Button>}
        </div> : <ul className={grid ? "grid grid-cols-[repeat(auto-fill,minmax(min(100%,220px),1fr))] gap-3" : "space-y-2"}>
          {files.map((file, index) => <li key={file.blobKey} className={`min-w-0 rounded-card border p-3 ${selection.has(file.blobKey) ? "border-primary bg-primary/10 ring-1 ring-primary/30" : "border-border bg-elevated"} ${grid ? "space-y-3" : "flex flex-wrap items-center gap-3"}`}>
            <div className={`flex items-start gap-3 ${grid ? "w-full" : "shrink-0"}`}>
              <FileCheckbox label={`Select ${file.name}`} checked={selection.has(file.blobKey)}
                disabled={file.state === "deleting" && !selection.has(file.blobKey)} onChange={() => toggleFile(file)} />
              <button type="button" className={`rounded-input focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary ${grid ? "w-full" : ""}`}
                aria-label={`Preview ${file.name}`} disabled={file.size > maxPreviewBytes} onClick={() => setDialog({ kind: "preview", files, index })}>
                <FileThumbnail file={file} grid={grid} revision={revision} maxBytes={maxPreviewBytes} />
              </button>
            </div>
            <div className="min-w-0 flex-1">
              <p className="break-words text-sm font-medium">{file.name}</p>
              <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted">
                <span>{formatSize(file.size)}</span>
                {file.width > 0 && <span>{file.width} x {file.height}</span>}
                <span>{new Date(file.createdAt).toLocaleDateString()}</span>
              </div>
              <p className="text-xs mt-1">
                {file.state === "deleting" ? "Deletion pending" : query.usage === "unused" && file.isInUse ? "Eligible for cleanup" : file.isInUse ? "In use" : "Unused"}
              </p>
              {file.lastError && <p className="text-xs text-danger mt-1 break-words">{file.lastError}</p>}
              {file.size > maxPreviewBytes && <p className="mt-1 text-xs text-warning">
                Too large to preview or download here. Use a backup export for the original.
              </p>}
            </div>
            <div className="flex w-full flex-wrap items-center gap-1 sm:w-auto" role="group" aria-label={`Actions for ${file.name}`}>
              <button type="button" className={actionClass} onClick={() => void copy(file)}>Copy link</button>
              {file.size <= maxPreviewBytes && <a className={actionClass} href={fileDownloadURL(file.blobKey)} download={file.name}>Download</a>}
              <button type="button" className={actionClass} onClick={() => setDialog({ kind: "usage", files: [file] })}>Usage</button>
              <button type="button" className={actionClass} disabled={file.state === "deleting"} onClick={() => setDialog({ kind: "rename", file })}>Rename</button>
              <button type="button" className={`${actionClass} text-danger`} disabled={file.state === "deleting" && !file.lastError}
                onClick={() => setDialog({ kind: "delete", files: [file], scope: deletionScope })}>{file.state === "deleting" ? "Retry deletion" : "Delete"}</button>
            </div>
          </li>)}
        </ul>}

        <div className="flex flex-wrap items-center justify-between gap-3 text-sm">
          <div role="status" className="flex flex-wrap gap-x-3 gap-y-1">
            <span>{data.total === 0 ? "0 images" : `${(data.page - 1) * data.pageSize + 1}-${Math.min(data.page * data.pageSize, data.total)} of ${data.total} images`}</span>
            {data.totalBytes !== undefined && <span>{formatSize(data.totalBytes)}</span>}
          </div>
          <FilePagination page={data.page} totalPages={data.totalPages} onChange={changePage} />
        </div>
      </>}

      <UploadFileDialog isOpen={dialog?.kind === "upload"} onClose={closeDialog} onUploaded={refresh} />
      {dialog?.kind === "rename" && <RenameFileDialog file={dialog.file} onClose={closeDialog} onRenamed={refresh} />}
      {dialog?.kind === "usage" && <FileUsageDialog files={dialog.files} onClose={closeDialog} />}
      {dialog?.kind === "delete" && <DeleteFilesDialog files={dialog.files} scope={dialog.scope} onClose={closeDialog} onChanged={deleted} />}
      {dialog?.kind === "prune" && <PruneFilesDialog request={dialog.request} count={dialog.count} onClose={closeDialog} onChanged={refresh} />}
      {selection.size > 0 && (
        <div className="fixed bottom-4 left-1/2 z-40 w-max max-w-[calc(100vw-2rem)] -translate-x-1/2 max-md:bottom-20">
          <div className="flex flex-wrap items-center justify-center gap-2 rounded-card border border-border-strong bg-elevated px-4 py-3 shadow-xl">
            <span className="mr-2 text-sm font-medium tabular-nums">{selection.size} selected</span>
            {readySelection.length < selection.size && <span className="text-xs text-muted">{selection.size - readySelection.length} deleting</span>}
            <Button size="small" loading={false} onClick={() => setDialog({ kind: "selection" })}>Review selected</Button>
            <Button size="small" variant="danger" loading={false} disabled={readySelection.length === 0}
              onClick={() => setDialog({ kind: "delete", files: readySelection, scope: deletionScope })}>Delete selected</Button>
            <Button size="small" variant="tertiary" onClick={() => setSelection(new Map())}>Clear selection</Button>
          </div>
        </div>
      )}
      {dialog?.kind === "selection" && (
        <FileDialog title={`Selected images (${selection.size})`} large onClose={closeDialog} footer={<>
          <Button disabled={downloadableFiles.length === 0} onClick={downloadSelected}>Download selected</Button>
          <Button disabled={selectedFiles.length === 0} onClick={() => setDialog({ kind: "usage", files: selectedFiles })}>View selected usage</Button>
          <Button variant="danger" disabled={readySelection.length === 0}
            onClick={() => setDialog({ kind: "delete", files: readySelection, scope: deletionScope })}>Delete selected</Button>
          <Button onClick={closeDialog}>Continue browsing</Button>
        </>}>
          <p className="mb-3 text-sm text-muted">Your selection is kept across pages and filters.</p>
          {selectedFiles.length === 0 ? <p>No images selected.</p> : (
            <ul className="max-h-[55vh] space-y-2 overflow-y-auto">
              {selectedFiles.map((file, index) => (
                <li key={file.blobKey} className="flex items-center gap-3 rounded-card border border-border p-2">
                  <FileCheckbox label={`Keep ${file.name} selected`} checked onChange={() => toggleFile(file)} />
                  <button type="button" disabled={file.size > maxPreviewBytes} aria-label={`Preview selected ${file.name}`}
                    className="shrink-0 rounded-input focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary"
                    onClick={() => setDialog({ kind: "preview", files: selectedFiles, index })}>
                    <FileThumbnail file={file} grid={false} revision={revision} maxBytes={maxPreviewBytes} />
                  </button>
                  <div className="min-w-0 text-sm">
                    <p className="break-words font-medium">{file.name}</p>
                    <p className="text-muted">{formatSize(file.size)}{file.state === "deleting" && ", Deletion pending"}</p>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </FileDialog>
      )}
      {dialog?.kind === "preview" && preview && (
        <FileDialog title={preview.name} large onClose={closeDialog} footer={<>
          <Button disabled={dialog.index === 0} onClick={() => setDialog({ ...dialog, index: dialog.index - 1 })}>Previous image</Button>
          <span className="self-center text-sm tabular-nums">{dialog.index + 1} / {dialog.files.length}</span>
          <Button disabled={dialog.index === dialog.files.length - 1} onClick={() => setDialog({ ...dialog, index: dialog.index + 1 })}>Next image</Button>
          <FileCheckbox checked={selection.has(preview.blobKey)} disabled={preview.state === "deleting" && !selection.has(preview.blobKey)}
            onChange={() => toggleFile(preview)}>Select image</FileCheckbox>
          {preview.size <= maxPreviewBytes && <>
            <a href={fileDownloadURL(preview.blobKey, true)} target="_blank" rel="noopener noreferrer" className={actionClass}>Open full-size image</a>
            <a href={fileDownloadURL(preview.blobKey)} download={preview.name} className={actionClass}>Download</a>
          </>}
          <Button onClick={closeDialog}>Continue browsing</Button>
        </>}>
          {preview.size <= maxPreviewBytes ? (
            <img src={filePreviewURL(preview.blobKey)} alt={preview.name} className="max-w-full max-h-[65vh] object-contain mx-auto" />
          ) : <p>Too large to preview or download here. Use a backup export for the original.</p>}
        </FileDialog>
      )}
    </div>
  )
}

export default FileManagementPage
