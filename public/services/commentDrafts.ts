import { DiscussionComment, ImageUpload } from "@fider/models"
import { analytics } from "./analytics"
import { CommentSubmission } from "./discussion"
import { newSubmissionID } from "./postSubmission"
import { fileToBase64 } from "./utils"

export interface CommentFile {
  fileId: string
  fileName: string
  contentType: string
  file?: File
}

export type CommentAttachment = ImageUpload | CommentFile

export interface CommentDraft extends Omit<CommentSubmission, "attachments"> {
  scope: string
  state: "draft" | "pending"
  attachments: CommentAttachment[]
  updatedAt: number
}

interface CommentReceipt {
  scope: string
  submissionId: string
  state: "completed"
  updatedAt: number
  comment: DiscussionComment
}

export type SavedComment = CommentDraft | CommentReceipt

interface DraftWrite {
  value: SavedComment
  kind: "save" | "rejected" | "completed"
  replaces?: CommentDraft
  action?: "create" | "update"
  journaled: boolean
  failed: boolean
  writing?: Promise<SavedComment>
}

const journalPrefix = "fider-comment-draft:"
const pendingWrites = new Map<string, DraftWrite>()
const storageListeners = new Set<(scope: string, failed: boolean) => void>()
let database: Promise<IDBDatabase> | undefined
let pruned = false
let retryTimer: ReturnType<typeof setTimeout> | undefined

function warnBeforeUnload(event: BeforeUnloadEvent) {
  const unsafeToLeave = [...pendingWrites.values()].some(({ value: saved, journaled }) =>
    !journaled || (saved.state !== "completed" && saved.attachments.some((image) => "fileId" in image))
  )

  if (unsafeToLeave) {
    event.preventDefault()
    event.returnValue = ""
  }
}

function releaseWrite(submissionId: string) {
  pendingWrites.delete(submissionId)

  if (pendingWrites.size === 0) {
    window.removeEventListener("beforeunload", warnBeforeUnload)
  }
}

function openDatabase(): Promise<IDBDatabase> {
  if (!database) {
    database = new Promise<IDBDatabase>((resolve, reject) => {
      const request = indexedDB.open("fider-comment-drafts", 2)

      request.onupgradeneeded = (event) => {
        const drafts = event.oldVersion === 0
          ? request.result.createObjectStore("drafts", { keyPath: "submissionId" })
          : request.transaction!.objectStore("drafts")

        drafts.createIndex("scope", "scope")
        request.result.createObjectStore("files", { keyPath: "fileId" })
      }

      request.onsuccess = () => {
        request.result.onversionchange = () => {
          request.result.close()
          database = undefined
        }

        resolve(request.result)
      }

      request.onerror = () => {
        reject(request.error)
      }
    }).catch((error) => {
      database = undefined
      throw error
    })
  }

  return database
}

async function accessDrafts<T>(
  mode: IDBTransactionMode,
  operation: (store: IDBObjectStore, transaction: IDBTransaction) => IDBRequest<T>
): Promise<T> {
  const connection = await openDatabase()

  return new Promise((resolve, reject) => {
    const stores = mode === "readwrite" ? ["drafts", "files"] : ["drafts"]
    const transaction = connection.transaction(stores, mode)
    const request = operation(transaction.objectStore("drafts"), transaction)

    transaction.oncomplete = () => resolve(request.result)
    transaction.onerror = () => reject(transaction.error)
    transaction.onabort = () => reject(transaction.error)
  })
}

function journalKey(value: SavedComment): string {
  return `${journalPrefix}${value.scope}:${value.submissionId}`
}

function metadata(value: SavedComment): SavedComment {
  if (value.state === "completed") {
    return value
  }

  return {
    ...value,
    attachments: value.attachments.map((image) => {
      if (!("fileId" in image)) {
        return image
      }

      return { fileId: image.fileId, fileName: image.fileName, contentType: image.contentType }
    }),
  }
}

function journal(value: SavedComment): boolean {
  // Navigation can cancel IndexedDB work; newly selected image bytes stay outside this synchronous journal.
  try {
    localStorage.setItem(journalKey(value), JSON.stringify(metadata(value)))
    return true
  } catch {
    return false
  }
}

async function restoreFiles(draft: CommentDraft): Promise<CommentDraft> {
  if (!draft.attachments.some((image) => "fileId" in image && !image.file)) {
    return draft
  }

  const connection = await openDatabase()
  const store = connection.transaction("files", "readonly").objectStore("files")
  const attachments = await Promise.all(draft.attachments.map((image) => {
    if (!("fileId" in image) || image.file) {
      return image
    }

    return new Promise<CommentFile>((resolve, reject) => {
      const request = store.get(image.fileId)

      request.onsuccess = () => {
        const file: File | undefined = request.result?.file
        resolve({ ...image, file })
      }

      request.onerror = () => reject(request.error)
    })
  }))

  return { ...draft, attachments }
}

async function pruneDrafts() {
  if (pruned) {
    return
  }

  const expiresBefore = Date.now() - 7 * 86400000
  const references = new Set<string>()

  const retainFiles = (saved: SavedComment) => {
    if (saved.state !== "completed") {
      for (const image of saved.attachments) {
        if ("fileId" in image) {
          references.add(image.fileId)
        }
      }
    }
  }

  try {
    for (let index = localStorage.length - 1; index >= 0; index--) {
      const key = localStorage.key(index)!

      if (key.startsWith(journalPrefix)) {
        const saved: SavedComment = JSON.parse(localStorage.getItem(key)!)

        if (saved.state === "completed" && saved.updatedAt < expiresBefore) {
          localStorage.removeItem(key)
        } else {
          retainFiles(saved)
        }
      }
    }
  } catch {
    // An inaccessible journal cannot establish that an old file is unreferenced.
    return
  }

  for (const pending of pendingWrites.values()) {
    retainFiles(pending.value)
  }

  const connection = await openDatabase()

  await new Promise<void>((resolve, reject) => {
    const transaction = connection.transaction(["drafts", "files"], "readwrite")
    const drafts = transaction.objectStore("drafts")
    const request = drafts.getAll()

    request.onsuccess = () => {
      for (const saved of request.result as SavedComment[]) {
        if (saved.state === "completed" && saved.updatedAt < expiresBefore) {
          drafts.delete(saved.submissionId)
        } else {
          retainFiles(saved)
        }
      }

      const files = transaction.objectStore("files").openCursor()

      files.onsuccess = () => {
        const cursor = files.result

        if (cursor) {
          if (cursor.value.createdAt < expiresBefore && !references.has(cursor.value.fileId)) {
            cursor.delete()
          }

          cursor.continue()
        }
      }
    }

    transaction.oncomplete = () => resolve()
    transaction.onerror = () => reject(transaction.error)
    transaction.onabort = () => reject(transaction.error)
  })

  pruned = true
}

function forgetReplaced(replaces: CommentDraft | undefined, next: CommentDraft) {
  if (!replaces || replaces.submissionId === next.submissionId) {
    return
  }

  const previous = metadata(replaces)

  try {
    const key = journalKey(replaces)

    if (localStorage.getItem(key) === JSON.stringify(previous)) {
      localStorage.removeItem(key)
    }
  } catch {
    // The retained copy remains available while browser storage is inaccessible.
  }

  const memory = pendingWrites.get(replaces.submissionId)?.value

  if (memory?.state === "draft" && JSON.stringify(metadata(memory)) === JSON.stringify(previous)) {
    releaseWrite(replaces.submissionId)
  }
}

function recover(scope: string): SavedComment[] {
  const values = new Map<string, SavedComment>()

  try {
    const prefix = `${journalPrefix}${scope}:`

    for (let index = 0; index < localStorage.length; index++) {
      const key = localStorage.key(index)!

      if (key.startsWith(prefix)) {
        const saved: SavedComment = JSON.parse(localStorage.getItem(key)!)
        values.set(saved.submissionId, saved)
      }
    }
  } catch {
    // In-memory intent also survives composer unmounts when storage is unavailable.
  }

  for (const { value: saved } of pendingWrites.values()) {
    if (saved.scope === scope) {
      values.set(saved.submissionId, saved)
    }
  }

  return [...values.values()].sort((left, right) => right.updatedAt - left.updatedAt)
}

function notifyStorage(scope: string) {
  const failed = [...pendingWrites.values()].some((write) => write.value.scope === scope && write.failed)

  for (const listener of storageListeners) {
    listener(scope, failed)
  }
}

function retryWrites() {
  if (retryTimer) {
    return
  }

  retryTimer = setTimeout(() => {
    retryTimer = undefined

    for (const write of pendingWrites.values()) {
      if (write.failed) {
        void persist(write).catch(() => {})
      }
    }
  }, 2000)
}

function persist(write: DraftWrite): Promise<SavedComment> {
  if (write.writing) {
    return write.writing
  }

  const { value, kind, replaces } = write

  write.writing = (async () => {
    try {
      let accepted = value
      let newlyCompleted = false

      await accessDrafts("readwrite", (store, transaction) => {
        const request = store.get(value.submissionId)

        request.onsuccess = () => {
          const saved: SavedComment | undefined = request.result

          if (pendingWrites.get(value.submissionId) !== write) {
            accepted = saved || value
            return
          }

          if (saved && (saved.state === "completed" || (kind === "save" && (
            saved.state === "pending" || saved.updatedAt > value.updatedAt
          )))) {
            accepted = saved
            return
          }

          // Pruning shouldnt remove an image before its draft becomes durable
          if (value.state !== "completed") {
            const files = transaction.objectStore("files")

            for (const image of value.attachments) {
              if (!("fileId" in image) || !image.file) {
                continue
              }

              const existing = files.getKey(image.fileId)

              existing.onsuccess = () => {
                if (existing.result === undefined) {
                  files.put({ fileId: image.fileId, file: image.file, createdAt: Date.now() })
                }
              }
            }
          }

          store.put(metadata(value))
          newlyCompleted = value.state === "completed"

          if (replaces && replaces.submissionId !== value.submissionId) {
            const original = store.get(replaces.submissionId)

            original.onsuccess = () => {
              const stored: SavedComment | undefined = original.result

              if (stored?.state === "draft" && JSON.stringify(stored) === JSON.stringify(metadata(replaces))) {
                store.delete(replaces.submissionId)
              }
            }
          }
        }

        return request
      })

      if (accepted.state !== "completed") {
        accepted = await restoreFiles(accepted)
      }

      if (pendingWrites.get(value.submissionId) === write) {
        if (!write.journaled || accepted !== value) {
          journal(accepted)
        }

        if (replaces && accepted.state !== "completed") {
          forgetReplaced(replaces, accepted)
        }

        releaseWrite(value.submissionId)
        notifyStorage(value.scope)
      }

      if (newlyCompleted && write.action) {
        analytics.event("comment", write.action)
      }

      return accepted
    } catch (cause) {
      write.failed = true
      notifyStorage(value.scope)
      retryWrites()
      throw cause
    } finally {
      write.writing = undefined
    }
  })()

  return write.writing
}

function retain(write: Omit<DraftWrite, "journaled" | "failed" | "writing">): Promise<SavedComment> {
  const pending: DraftWrite = { ...write, journaled: journal(write.value), failed: false }

  if (pendingWrites.size === 0) {
    window.addEventListener("beforeunload", warnBeforeUnload)
  }

  pendingWrites.set(write.value.submissionId, pending)

  if (pending.journaled && write.replaces && write.value.state !== "completed") {
    forgetReplaced(write.replaces, write.value)
  }

  return persist(pending)
}

export const commentDrafts = {
  recover,

  subscribe: (scope: string, onStorageError: (failed: boolean) => void): (() => void) => {
    const listener = (changedScope: string, failed: boolean) => {
      if (changedScope === scope) {
        onStorageError(failed)
      }
    }

    storageListeners.add(listener)
    notifyStorage(scope)

    return () => {
      storageListeners.delete(listener)
    }
  },

  attach: (file: File): CommentFile => ({
    fileId: newSubmissionID(),
    fileName: file.name,
    contentType: file.type,
    file,
  }),

  load: async (scope: string): Promise<CommentDraft[]> => {
    await pruneDrafts()
    const values = new Map(recover(scope).map((saved) => [saved.submissionId, saved]))
    const stored: SavedComment[] = await accessDrafts("readonly", (store) => store.index("scope").getAll(scope))

    for (const saved of stored) {
      const local = values.get(saved.submissionId)

      if (!local || saved.state === "completed" || (!pendingWrites.has(saved.submissionId) && local.state !== "completed" && (
        saved.state === "pending" || (local.state === "draft" && saved.updatedAt > local.updatedAt)
      ))) {
        values.set(saved.submissionId, saved)
      }
    }

    const drafts = [...values.values()].filter((saved): saved is CommentDraft => saved.state !== "completed")

    return Promise.all(drafts.sort((left, right) => right.updatedAt - left.updatedAt).map(restoreFiles))
  },

  prepare: async (draft: CommentDraft): Promise<CommentSubmission> => ({
    submissionId: draft.submissionId,
    parentId: draft.parentId,
    content: draft.content,
    attachments: await Promise.all(draft.attachments.map(async (image): Promise<ImageUpload> => {
      if (!("fileId" in image)) {
        return image
      }

      if (!image.file) {
        throw new Error(`The saved image “${image.fileName}” is unavailable. Please select it again.`)
      }

      return {
        remove: false,
        upload: { fileName: image.fileName, contentType: image.contentType, content: await fileToBase64(image.file) },
      }
    })),
  }),

  save: (draft: CommentDraft, replaces?: CommentDraft): Promise<SavedComment> =>
    retain({ value: draft, kind: "save", replaces }),

  rejected: (draft: CommentDraft): Promise<SavedComment> =>
    retain({ value: { ...draft, state: "draft" }, kind: "rejected" }),

  complete: async (draft: CommentDraft, comment: DiscussionComment, action: "create" | "update"): Promise<void> => {
    const receipt: CommentReceipt = {
      scope: draft.scope,
      submissionId: draft.submissionId,
      state: "completed",
      updatedAt: Date.now(),
      comment,
    }

    await retain({ value: receipt, kind: "completed", action })
  },
}
