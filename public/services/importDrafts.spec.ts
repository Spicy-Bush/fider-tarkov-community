import { readSavedDrafts } from "./importDrafts"

jest.mock("./postSubmission", () => ({ newSubmissionID: () => "session-draft" }))

type SavedStore = Map<string, any>
const databases = new Map<string, Record<string, SavedStore>>()
const originalIndexedDB = Object.getOwnPropertyDescriptor(window, "indexedDB")
const originalKeyRange = Object.getOwnPropertyDescriptor(window, "IDBKeyRange")

function request<T>(result: T) {
  const value = { result, onsuccess: undefined as (() => void) | undefined }
  queueMicrotask(() => value.onsuccess?.())
  return value
}

beforeEach(() => {
  databases.clear()
  localStorage.clear()
  sessionStorage.clear()

  Object.defineProperty(window, "IDBKeyRange", {
    configurable: true,
    value: { bound: (lower: string, upper: string) => ({ lower, upper }) },
  })
  Object.defineProperty(window, "indexedDB", {
    configurable: true,
    value: {
      open: (name: string) => {
        const stores = databases.get(name)
        const connection = {
          close: jest.fn(),
          objectStoreNames: { contains: (store: string) => !!stores?.[store] },
          transaction: () => {
            const transaction = {
              oncomplete: undefined as (() => void) | undefined,
              objectStore: (store: string) => ({
                getAll: (range?: { lower: string; upper: string }) => request([...stores![store]]
                  .filter(([key]) => !range || (key >= range.lower && key <= range.upper))
                  .map(([, value]) => value)),
                get: (key: string) => request(stores![store].get(key)),
                delete: (key: string) => stores![store].delete(key),
              }),
            }
            setTimeout(() => transaction.oncomplete?.(), 0)
            return transaction
          },
        }
        const opened = {
          result: connection,
          onsuccess: undefined as (() => void) | undefined,
          onerror: undefined as (() => void) | undefined,
          onupgradeneeded: undefined as (() => void) | undefined,
          transaction: { abort: () => queueMicrotask(() => opened.onerror?.()) },
        }
        queueMicrotask(() => stores ? opened.onsuccess?.() : opened.onupgradeneeded?.())
        return opened
      },
    },
  })
})

afterEach(() => {
  for (const [key, descriptor] of [["indexedDB", originalIndexedDB], ["IDBKeyRange", originalKeyRange]] as const) {
    if (descriptor) Object.defineProperty(window, key, descriptor)
    else delete (window as any)[key]
  }
})

function comment(submissionId: string, fields: Record<string, unknown> = {}) {
  return {
    submissionId,
    scope: "1:post:15:root:account:7",
    state: "draft",
    updatedAt: 10,
    content: "Saved text",
    attachments: [],
    ...fields,
  }
}

function journal(value: ReturnType<typeof comment>) {
  const key = `fider-comment-draft:${value.scope}:${value.submissionId}`
  localStorage.setItem(key, JSON.stringify(value))
  return key
}

test("import preserves pending requests, completed precedence, files, and account boundaries without a count limit", async () => {
  const file = new File(["image bytes"], "saved.png", { type: "image/png" })
  const image = { fileId: "shared-file", fileName: file.name, contentType: file.type }
  const posts = new Map<string, any>([
    ["1:7:post-pending", { submissionId: "post-pending", title: "Pending", description: "Description", attachments: [] }],
    ["1:7:post-rejected", { submissionId: "post-rejected", title: "Rejected", description: "Retain", attachments: [], rejection: {} }],
    ["1:7:post-complete", { submissionId: "post-complete", receipt: { number: 1, slug: "done" } }],
    ["1:8:other-account", { submissionId: "other-account" }],
  ])
  const comments = new Map<string, any>([
    ["pending", comment("pending", { state: "pending", content: "Sealed request", parentId: 13, attachments: [image] })],
    ["database-complete", comment("database-complete", { state: "completed" })],
    ["journal-complete", comment("journal-complete", { state: "pending" })],
    ["unowned", comment("unowned", { scope: "1:page:4:root", content: "Adopt this work" })],
    ["foreign", comment("foreign", { scope: "2:post:15:root:account:7" })],
    ["other-user", comment("other-user", { scope: "1:post:15:root:account:8" })],
  ])
  databases.set("fider-post-submissions", { pending: posts })
  databases.set("fider-comment-drafts", {
    drafts: comments,
    files: new Map([["shared-file", { fileId: "shared-file", file }]]),
  })
  journal(comment("pending", { updatedAt: 50, content: "Later editable text" }))
  journal(comment("database-complete", { state: "pending", updatedAt: 100 }))
  journal(comment("journal-complete", { state: "completed" }))
  journal(comment("newer", { updatedAt: 60, content: "Journal only" }))
  sessionStorage.setItem("PostInput-Title", "Unsaved title")
  sessionStorage.setItem("PostInput-Description", "Unsaved description")

  const imported = await readSavedDrafts("1:7")

  expect(imported.drafts).toHaveLength(6)
  expect(imported.drafts.find(value => value.id === "pending")).toMatchObject({
    phase: "pending",
    scope: "1:post:15:root",
    payload: { content: "Sealed request", parentId: 13, attachments: [{ kind: "local", fileId: image.fileId, file }] },
  })
  expect(imported.drafts.find(value => value.id === "post-pending")).toMatchObject({
    phase: "pending", payload: { title: "Pending", description: "Description", attachments: [] },
  })
  expect(imported.drafts.find(value => value.id === "post-rejected")?.phase).toBe("editable")
  expect(imported.drafts.find(value => value.id === "unowned")?.scope).toBe("1:page:4:root")
  expect(imported.drafts[0].updatedAt).toBeGreaterThan(imported.drafts.at(-1)!.updatedAt)
  expect(imported.drafts.some(value => "submissionId" in value.payload)).toBe(false)
  expect(posts.size).toBe(4)
  expect(comments.size).toBe(6)
  expect(sessionStorage.getItem("PostInput-Title")).toBe("Unsaved title")
})

test("cleanup removes only unchanged imported records and keeps files used by another account or a newer journal", async () => {
  const image = (id: string) => ({ fileId: id, fileName: `${id}.png`, contentType: "image/png" })
  const saved = comment("saved", { attachments: [image("shared"), image("only-imported"), image("journal-shared")] })
  const changed = comment("changed")
  const other = comment("other", { scope: "1:post:15:root:account:8", attachments: [image("shared")] })
  const comments = new Map([[saved.submissionId, saved], [changed.submissionId, changed], [other.submissionId, other]])
  const files = new Map(["shared", "only-imported", "journal-shared"].map(fileId => [fileId, { fileId }]))
  const post = { submissionId: "post", title: "Post", description: "Text", attachments: [] }
  const posts = new Map([["1:7:post", post]])
  databases.set("fider-post-submissions", { pending: posts })
  databases.set("fider-comment-drafts", { drafts: comments, files })
  const savedKey = journal(saved)
  sessionStorage.setItem("PostInput-Title", "Original")

  const imported = await readSavedDrafts("1:7")
  comments.set("changed", { ...changed, content: "Changed during import" })
  posts.set("1:7:post", { ...post, title: "Changed during import" })
  journal(comment("later", { attachments: [image("journal-shared")] }))
  sessionStorage.setItem("PostInput-Title", "Newer title")
  await imported.cleanup()

  expect([...comments.keys()]).toEqual(["changed", "other"])
  expect(posts.get("1:7:post")?.title).toBe("Changed during import")
  expect([...files.keys()]).toEqual(["shared", "journal-shared"])
  expect(localStorage.getItem(savedKey)).toBeNull()
  expect(sessionStorage.getItem("PostInput-Title")).toBe("Newer title")
})

test("absent stores and malformed journals do not create databases or discard readable work", async () => {
  localStorage.setItem("fider-comment-draft:broken", "{")
  journal(comment("readable"))

  const imported = await readSavedDrafts("1:7")
  expect(imported.drafts.map(value => value.id)).toEqual(["readable"])
  await imported.cleanup()

  expect(databases.size).toBe(0)
  expect(localStorage.getItem("fider-comment-draft:broken")).toBe("{")
})
