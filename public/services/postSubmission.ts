import { ImageUpload, Post } from "@fider/models"
import { createPost } from "./actions/post"
import { analytics } from "./analytics"
import { Failure, RequestError, Result } from "./http"

export interface PendingPostSubmission {
  submissionId: string
  title: string
  description: string
  attachments: ImageUpload[]
  rejection?: Failure
}

export interface CompletedPostSubmission {
  submissionId: string
  completedAt: number
  receipt: Pick<Post, "number" | "slug">
}

export type SavedPostSubmission = PendingPostSubmission | CompletedPostSubmission

export class SubmissionStorageError extends Error {
  constructor(readonly cause: unknown) {
    super("Could not access saved submissions.")
  }
}

async function pendingSubmission<T>(
  mode: IDBTransactionMode,
  operation: (store: IDBObjectStore) => IDBRequest<T>
): Promise<T> {
  try {
    return await accessSubmissions(mode, operation)
  } catch (cause) {
    throw new SubmissionStorageError(cause)
  }
}

async function accessSubmissions<T>(
  mode: IDBTransactionMode,
  operation: (store: IDBObjectStore) => IDBRequest<T>
): Promise<T> {
  const database = await new Promise<IDBDatabase>((resolve, reject) => {
    const request = indexedDB.open("fider-post-submissions", 1)
    request.onupgradeneeded = () => request.result.createObjectStore("pending")
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error)
  })

  try {
    return await new Promise((resolve, reject) => {
      const transaction = database.transaction("pending", mode)
      const request = operation(transaction.objectStore("pending"))

      transaction.oncomplete = () => resolve(request.result)
      transaction.onerror = () => reject(transaction.error)
      transaction.onabort = () => reject(transaction.error)
    })
  } finally {
    database.close()
  }
}

export const postSubmissions = {
  load: async (account: string): Promise<SavedPostSubmission[]> => {
    const expiresBefore = Date.now() - 7 * 86400000
    const values: SavedPostSubmission[] = await pendingSubmission("readwrite", (store) => {
      const request = store.getAll(IDBKeyRange.bound(`${account}:`, `${account}:\uffff`))

      request.onsuccess = () => {
        for (const value of request.result as SavedPostSubmission[]) {
          if ("receipt" in value && value.completedAt < expiresBefore) {
            store.delete(`${account}:${value.submissionId}`)
          }
        }
      }

      return request
    })

    return values.filter((value) => !("receipt" in value) || value.completedAt >= expiresBefore)
  },

  save: async (account: string, value: PendingPostSubmission, replaces?: string): Promise<SavedPostSubmission> => {
    const saved: SavedPostSubmission | undefined = await pendingSubmission("readwrite", (store) => {
      const key = `${account}:${value.submissionId}`
      const request = store.get(key)

      request.onsuccess = () => {
        if (request.result && "receipt" in request.result) {
          return
        }

        store.put(value, key)

        if (replaces && replaces !== value.submissionId) {
          const replacedKey = `${account}:${replaces}`
          const previous = store.get(replacedKey)

          previous.onsuccess = () => {
            const submission: SavedPostSubmission | undefined = previous.result

            if (submission && !("receipt" in submission) && submission.rejection) {
              store.delete(replacedKey)
            }
          }
        }
      }

      return request
    })

    return saved && "receipt" in saved ? saved : value
  },

  updateAttachments: (account: string, submissionId: string, attachments: ImageUpload[]) =>
    pendingSubmission("readwrite", (store) => {
      const key = `${account}:${submissionId}`
      const request = store.get(key)

      request.onsuccess = () => {
        const saved: SavedPostSubmission | undefined = request.result

        if (saved && !("receipt" in saved) && saved.rejection) {
          store.put({ ...saved, attachments }, key)
        }
      }

      return request
    }),

  complete: async (account: string, submissionId: string, receipt: Pick<Post, "number" | "slug">): Promise<void> => {
    const saved: SavedPostSubmission | undefined = await pendingSubmission("readwrite", (store) => {
      const key = `${account}:${submissionId}`
      const request = store.get(key)

      request.onsuccess = () => {
        if (request.result && "receipt" in request.result) {
          return
        }

        const completed: CompletedPostSubmission = {
          submissionId,
          receipt: { number: receipt.number, slug: receipt.slug },
          completedAt: Date.now(),
        }

        store.put(completed, key)
      }

      return request
    })

    if (!saved || !("receipt" in saved)) {
      analytics.event("post", "create")
    }
  },
}

export async function sendPostSubmission(
  submission: PendingPostSubmission,
  signal: AbortSignal
): Promise<Result<Pick<Post, "number" | "slug">>> {
  for (let attempt = 0; ; attempt++) {
    if (signal.aborted) {
      throw signal.reason
    }

    try {
      const result = await createPost(submission, signal)

      if (result.ok || (result.status && result.status < 500) || attempt === 2) {
        return result
      }
    } catch (cause) {
      if (!(cause instanceof RequestError) || attempt === 2 || signal.aborted) {
        throw cause
      }
    }

    await new Promise<void>((resolve, reject) => {
      const abort = () => {
        clearTimeout(timer)
        reject(signal.reason)
      }

      const timer = setTimeout(() => {
        signal.removeEventListener("abort", abort)
        resolve()
      }, 500 * (attempt + 1))

      signal.addEventListener("abort", abort, { once: true })

      if (signal.aborted) {
        abort()
      }
    })
  }
}

export function newSubmissionID(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(16))

  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("")
}
