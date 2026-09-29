import type { AccountDraft } from "./browserDrafts"
import { restoreDraftImage, SavedDraftImage } from "./draftImages"
import { newSubmissionID } from "./postSubmission"

interface SavedPost {
  submissionId: string
  title: string
  description: string
  attachments: SavedDraftImage[]
  rejection?: unknown
  receipt?: unknown
}

interface SavedComment {
  submissionId: string
  scope: string
  state: "draft" | "pending" | "completed"
  updatedAt: number
  content: string
  attachments: SavedDraftImage[]
  parentId?: number
}

const journalPrefix = "fider-comment-draft:"

function openSavedDatabase(name: string): Promise<IDBDatabase | undefined> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(name)
    let absent = false

    request.onupgradeneeded = () => {
      absent = true
      request.transaction!.abort()
    }

    request.onsuccess = () => resolve(request.result)
    request.onerror = () => absent ? resolve(undefined) : reject(request.error)
  })
}

function readRequest<T>(request: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error)
  })
}

function finishTransaction(transaction: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    transaction.oncomplete = () => resolve()
    transaction.onerror = () => reject(transaction.error)
    transaction.onabort = () => reject(transaction.error)
  })
}

function readJournal(): Map<string, { saved?: SavedComment; text: string }> {
  const entries = new Map<string, { saved?: SavedComment; text: string }>()

  for (let index = 0; index < localStorage.length; index++) {
    const key = localStorage.key(index)!
    if (!key.startsWith(journalPrefix)) continue

    const text = localStorage.getItem(key)!
    try {
      entries.set(key, { saved: JSON.parse(text), text })
    } catch {
      entries.set(key, { text })
    }
  }

  return entries
}

export async function readSavedDrafts(account: string): Promise<{
  drafts: AccountDraft[]
  cleanup: () => Promise<void>
}> {
  const [tenant, user] = account.split(":")
  const ownedScopeSuffix = `:account:${user}`
  const commentScope = (scope: string): string | undefined => {
    if (!scope.startsWith(`${tenant}:`)) return undefined
    if (!scope.includes(":account:")) return scope
    if (scope.endsWith(ownedScopeSuffix)) return scope.slice(0, -ownedScopeSuffix.length)
    return undefined
  }

  const drafts: AccountDraft[] = []
  const posts = new Map<string, SavedPost>()
  const storedComments = new Map<string, SavedComment>()
  const journal = new Map<string, string>()
  const files = new Set<string>()
  const now = Date.now()

  const postDatabase = await openSavedDatabase("fider-post-submissions")
  if (postDatabase) {
    try {
      const store = postDatabase.transaction("pending", "readonly").objectStore("pending")
      const values: SavedPost[] = await readRequest(store.getAll(IDBKeyRange.bound(`${account}:`, `${account}:\uffff`)))

      for (const saved of values) {
        if (saved.receipt) continue

        posts.set(`${account}:${saved.submissionId}`, saved)
        const payload = { title: saved.title, description: saved.description, attachments: saved.attachments.map(restoreDraftImage) }
        drafts.push({
          id: saved.submissionId,
          revision: 1,
          kind: "post",
          phase: saved.rejection ? "editable" : "pending",
          scope: "post:new",
          payload,
          updatedAt: now,
        })
      }
    } finally {
      postDatabase.close()
    }
  }

  const comments = new Map<string, SavedComment>()
  for (const [key, { saved, text }] of readJournal()) {
    if (!saved || !commentScope(saved.scope)) continue

    journal.set(key, text)
    const previous = comments.get(saved.submissionId)
    if (!previous || saved.state === "completed" || (previous.state !== "completed" && (
      saved.state === "pending" || (previous.state === "draft" && saved.updatedAt > previous.updatedAt)
    ))) {
      comments.set(saved.submissionId, saved)
    }
  }

  const commentDatabase = await openSavedDatabase("fider-comment-drafts")
  try {
    if (commentDatabase) {
      const store = commentDatabase.transaction("drafts", "readonly").objectStore("drafts")
      const values: SavedComment[] = await readRequest(store.getAll())

      for (const saved of values) {
        if (!commentScope(saved.scope)) continue

        storedComments.set(saved.submissionId, saved)
        const local = comments.get(saved.submissionId)
        if (!local || saved.state === "completed" || (local.state !== "completed" && (
          saved.state === "pending" || (local.state === "draft" && saved.updatedAt > local.updatedAt)
        ))) {
          comments.set(saved.submissionId, saved)
        }
      }
    }

    const fileStore = commentDatabase?.objectStoreNames.contains("files")
      ? commentDatabase.transaction("files", "readonly").objectStore("files")
      : undefined
    const fileReads = new Map<string, Promise<File | undefined>>()

    for (const saved of comments.values()) {
      if (saved.state === "completed") continue

      for (const image of saved.attachments) {
        if (!("fileId" in image)) continue

        files.add(image.fileId)
        if (!image.file && fileStore && !fileReads.has(image.fileId)) {
          fileReads.set(image.fileId, readRequest(fileStore.get(image.fileId)).then(value => value?.file))
        }
      }
    }

    for (const saved of comments.values()) {
      if (saved.state === "completed") continue

      const attachments = await Promise.all(saved.attachments.map(async image => {
        if (!("fileId" in image) || image.file) return restoreDraftImage(image)
        return restoreDraftImage({ ...image, file: await fileReads.get(image.fileId) })
      }))
      const payload = { content: saved.content, parentId: saved.parentId, attachments }

      drafts.push({
        id: saved.submissionId,
        revision: 1,
        kind: "comment",
        phase: saved.state === "pending" ? "pending" : "editable",
        scope: commentScope(saved.scope)!,
        payload,
        updatedAt: saved.updatedAt,
      })
    }
  } finally {
    commentDatabase?.close()
  }

  const title = sessionStorage.getItem("PostInput-Title")
  const description = sessionStorage.getItem("PostInput-Description")
  if (title || description) {
    const payload = { title: title || "", description: description || "", attachments: [] }
    drafts.push({
      id: newSubmissionID(),
      revision: 1,
      kind: "post",
      phase: "editable",
      scope: "post:new",
      payload,
      updatedAt: now,
    })
  }

  return {
    drafts: drafts.sort((left, right) => right.updatedAt - left.updatedAt),
    async cleanup() {
      const postsDatabase = await openSavedDatabase("fider-post-submissions")
      if (postsDatabase) {
        try {
          const transaction = postsDatabase.transaction("pending", "readwrite")
          const complete = finishTransaction(transaction)
          const store = transaction.objectStore("pending")

          for (const [key, saved] of posts) {
            const read = store.get(key)
            read.onsuccess = () => {
              if (JSON.stringify(read.result) === JSON.stringify(saved)) store.delete(key)
            }
          }

          await complete
        } finally {
          postsDatabase.close()
        }
      }

      for (const [key, saved] of journal) {
        if (localStorage.getItem(key) === saved) localStorage.removeItem(key)
      }

      const references = new Set<string>()
      const retainFiles = (saved: SavedComment) => {
        if (saved.state === "completed") return

        for (const image of saved.attachments) {
          if ("fileId" in image) references.add(image.fileId)
        }
      }

      let unreadableJournal = false
      for (const { saved } of readJournal().values()) {
        if (saved) retainFiles(saved)
        else unreadableJournal = true
      }

      const commentsDatabase = await openSavedDatabase("fider-comment-drafts")
      if (commentsDatabase) {
        try {
          const hasFiles = commentsDatabase.objectStoreNames.contains("files")
          const transaction = commentsDatabase.transaction(hasFiles ? ["drafts", "files"] : "drafts", "readwrite")
          const complete = finishTransaction(transaction)
          const store = transaction.objectStore("drafts")
          const read = store.getAll()

          read.onsuccess = () => {
            for (const saved of read.result as SavedComment[]) {
              if (JSON.stringify(saved) === JSON.stringify(storedComments.get(saved.submissionId))) {
                store.delete(saved.submissionId)
              } else {
                retainFiles(saved)
              }
            }

            if (hasFiles && !unreadableJournal) {
              for (const id of files) {
                if (!references.has(id)) transaction.objectStore("files").delete(id)
              }
            }
          }

          await complete
        } finally {
          commentsDatabase.close()
        }
      }

      if (sessionStorage.getItem("PostInput-Title") === title) sessionStorage.removeItem("PostInput-Title")
      if (sessionStorage.getItem("PostInput-Description") === description) sessionStorage.removeItem("PostInput-Description")
    },
  }
}
