jest.mock("lib0/webcrypto", () => {
  const crypto = require("node:crypto").webcrypto
  return { getRandomValues: crypto.getRandomValues.bind(crypto), subtle: crypto.subtle }
})

import { act, renderHook, waitFor } from "@testing-library/react"
import * as Y from "yjs"
import { http } from "@fider/services/http"
import { accountDrafts, AccountDraft } from "@fider/services/browserDrafts"
import { changePageDocument, decodePageState, encodePageState, readPageDocument } from "@fider/services/pageCollaboration"
import { initialPageWorkingCopy, usePageCollaboration } from "./usePageCollaboration"

jest.mock("./use-fider", () => ({
  useFider: () => ({ session: { tenant: { id: 1 }, user: { id: 7, name: "Admin" }, isAuthenticated: true } }),
}))
jest.mock("@fider/services/browserDrafts", () => ({
  accountDrafts: { list: jest.fn(), save: jest.fn(), remove: jest.fn(), markUnsaved: jest.fn() },
}))
jest.mock("@fider/services/postSubmission", () => {
  let id = 0
  return { newSubmissionID: () => `page-${++id}` }
})

class Source {
  static connections: Source[] = []
  onopen?: () => void
  onmessage?: (event: { data: string }) => void
  onerror?: () => void
  close = jest.fn()

  constructor() {
    Source.connections.push(this)
  }
}

const page = { id: 14 } as any
const records = new Map<string, AccountDraft<any>>()
let server: Y.Doc
let loseReply = false
let rejectContent = false
let holdReply: Promise<void> | undefined

function deferred() {
  let resolve!: () => void
  const promise = new Promise<void>(done => { resolve = done })
  return { promise, resolve }
}

beforeEach(() => {
  jest.clearAllMocks()
  records.clear()
  Source.connections = []
  window.EventSource = Source as any
  server = new Y.Doc()
  changePageDocument(server, { ...initialPageWorkingCopy(), title: "Original", content: "First\nSecond" }, "server")
  loseReply = false
  rejectContent = false
  holdReply = undefined

  jest.mocked(accountDrafts.list).mockImplementation(async (_account, scope) => [...records.values()].filter(draft => draft.scope === scope))
  jest.mocked(accountDrafts.save).mockImplementation(async (_account, draft) => {
    const saved = { ...draft, revision: draft.revision + 1, phase: "editable" as const, updatedAt: Date.now() }
    records.set(saved.id, saved)
    return { accepted: true, draft: saved }
  })
  jest.mocked(accountDrafts.remove).mockImplementation(async (_account, receipt) => {
    if (records.get(receipt.id)?.revision !== receipt.revision) {
      return false
    }

    return records.delete(receipt.id)
  })
  jest.spyOn(http, "post").mockImplementation(async (url, body) => {
    if (url === "/api/page-drafts") {
      return { ok: true, data: { pageId: 14, state: encodePageState(Y.encodeStateAsUpdate(server)), stateVector: encodePageState(Y.encodeStateVector(server)), legacyDrafts: [] } } as any
    }
    if (url.endsWith("/draft/cursor")) {
      return { ok: true, data: undefined } as any
    }

    if (!url.endsWith("/draft")) {
      throw new Error(`Unexpected request: ${url}`)
    }

    const candidate = new Y.Doc()
    Y.applyUpdate(candidate, Y.encodeStateAsUpdate(server))
    Y.applyUpdate(candidate, decodePageState(body.update))
    if (rejectContent && candidate.getText("content").toString().length > 25) {
      return { ok: false, status: 400, error: { errors: [{ message: "Content is too long." }] } }
    }
    Y.applyUpdate(server, decodePageState(body.update))
    const data = { update: encodePageState(Y.encodeStateAsUpdate(server, decodePageState(body.stateVector))), stateVector: encodePageState(Y.encodeStateVector(server)) }
    if (holdReply) {
      await holdReply
    }
    if (loseReply) {
      loseReply = false
      throw new Error("Response lost after commit")
    }
    return { ok: true, data } as any
  })
})

afterEach(() => {
  jest.restoreAllMocks()
  server.destroy()
})

test("a lost acknowledgement and reload recover the same accepted edit once", async () => {
  const first = renderHook(() => usePageCollaboration(page))
  await waitFor(() => expect(first.result.current.session).toBeDefined())

  act(() => first.result.current.change({ title: "Changed" }))
  loseReply = true
  await act(async () => {
    await expect(first.result.current.flush()).rejects.toThrow("Response lost")
  })
  expect(readPageDocument(server).title).toBe("Changed")
  await waitFor(() => expect(records.size).toBe(1))
  first.unmount()

  const second = renderHook(() => usePageCollaboration(page))
  await waitFor(() => expect(second.result.current.session).toBeDefined())
  await act(async () => second.result.current.flush())
  expect(second.result.current.value.title).toBe("Changed")
  expect(readPageDocument(server).title).toBe("Changed")
  second.unmount()
})

test("edits made while an acknowledgement is delayed are sent before flush completes", async () => {
  const editor = renderHook(() => usePageCollaboration(page))
  await waitFor(() => expect(editor.result.current.session).toBeDefined())
  const delayed = deferred()
  holdReply = delayed.promise
  act(() => editor.result.current.change({ title: "First edit" }))

  let flushing!: Promise<void>
  act(() => { flushing = editor.result.current.flush() })
  await waitFor(() => expect(readPageDocument(server).title).toBe("First edit"))
  act(() => editor.result.current.change({ title: "Later edit" }))
  holdReply = undefined
  await act(async () => {
    delayed.resolve()
    await flushing
  })

  expect(readPageDocument(server).title).toBe("Later edit")
  expect(editor.result.current.value.title).toBe("Later edit")
  editor.unmount()
})

test("two editors retain independent text edits and reconverge after reconnect", async () => {
  const first = renderHook(() => usePageCollaboration(page))
  const second = renderHook(() => usePageCollaboration(page))
  await waitFor(() => expect(first.result.current.session && second.result.current.session).toBeDefined())

  act(() => {
    first.result.current.change({ content: "First A\nSecond", topics: [1] })
    second.result.current.change({ content: "First\nSecond B", topics: [2] })
  })
  await act(async () => first.result.current.flush())
  await act(async () => second.result.current.flush())
  await act(async () => first.result.current.flush())

  expect(readPageDocument(server).content).toBe("First A\nSecond B")
  expect(first.result.current.value.topics).toEqual([1, 2])
  expect(second.result.current.value.content).toBe("First A\nSecond B")
  first.unmount()
  second.unmount()
})

test("a rejected limit can be corrected and recovered without replacing the collaboration document", async () => {
  rejectContent = true
  const editor = renderHook(() => usePageCollaboration(page))
  await waitFor(() => expect(editor.result.current.session).toBeDefined())
  const document = editor.result.current.session!.document

  act(() => editor.result.current.change({ content: "x".repeat(30) }))
  await act(async () => {
    await expect(editor.result.current.flush()).rejects.toMatchObject({ errors: [{ message: "Content is too long." }] })
  })
  expect(editor.result.current.value.content).toBe("x".repeat(30))
  expect(readPageDocument(server).content).toBe("First\nSecond")

  act(() => editor.result.current.change({ content: "Corrected" }))
  await act(async () => editor.result.current.flush())
  expect(readPageDocument(server).content).toBe("Corrected")
  expect(editor.result.current.session!.document).toBe(document)
  expect(editor.result.current.error).toBeUndefined()
  editor.unmount()
})

test("browser storage failure does not prevent a durable server save", async () => {
  jest.mocked(accountDrafts.save).mockRejectedValue(new Error("Quota exceeded"))
  const logging = jest.spyOn(console, "error").mockImplementation(() => {})
  const editor = renderHook(() => usePageCollaboration(page))
  await waitFor(() => expect(editor.result.current.session).toBeDefined())

  act(() => editor.result.current.change({ title: "Server copy survives" }))
  await waitFor(() => expect(editor.result.current.recoveryError).toBeDefined())
  await act(async () => editor.result.current.flush())

  expect(readPageDocument(server).title).toBe("Server copy survives")
  expect(editor.result.current.recoveryError).toBeUndefined()
  expect(logging).toHaveBeenCalled()
  editor.unmount()
})

test("retry loads previously unread recovery work without replacing edits made in the open editor", async () => {
  const recovery = new Y.Doc()
  Y.applyUpdate(recovery, Y.encodeStateAsUpdate(server))
  changePageDocument(recovery, { content: "Recovered content" }, "offline")
  records.set("unread", {
    id: "unread",
    revision: 1,
    kind: "page",
    phase: "editable",
    scope: "page-collaboration:14",
    updatedAt: Date.now(),
    payload: { state: Y.encodeStateAsUpdate(recovery) },
  })
  recovery.destroy()

  jest.mocked(accountDrafts.list).mockRejectedValueOnce(new Error("Database temporarily unavailable"))
  jest.spyOn(console, "error").mockImplementation(() => {})
  const editor = renderHook(() => usePageCollaboration(page))
  await waitFor(() => expect(editor.result.current.session).toBeDefined())

  act(() => editor.result.current.change({ title: "New work remains" }))
  await act(async () => editor.result.current.flush())
  expect(editor.result.current.recoveryError).toBeDefined()
  expect(readPageDocument(server).content).toBe("First\nSecond")

  await act(async () => editor.result.current.retry())
  expect(editor.result.current.recoveryError).toBeUndefined()
  expect(readPageDocument(server).title).toBe("New work remains")
  expect(readPageDocument(server).content).toBe("Recovered content")
  expect(records.has("unread")).toBe(false)
  editor.unmount()
})

test("unmount during a delayed acknowledgement keeps a later unsent edit for reopening", async () => {
  const first = renderHook(() => usePageCollaboration(page))
  await waitFor(() => expect(first.result.current.session).toBeDefined())
  const delayed = deferred()
  holdReply = delayed.promise

  act(() => first.result.current.change({ title: "Accepted before closing" }))
  let flushing!: Promise<void>
  act(() => { flushing = first.result.current.flush() })
  await waitFor(() => expect(readPageDocument(server).title).toBe("Accepted before closing"))
  act(() => first.result.current.change({ title: "Still unsent when closed" }))
  await waitFor(() => expect([...records.values()][0]?.revision).toBeGreaterThan(1))
  first.unmount()

  holdReply = undefined
  await act(async () => {
    delayed.resolve()
    await flushing
  })
  const second = renderHook(() => usePageCollaboration(page))
  await waitFor(() => expect(second.result.current.session).toBeDefined())
  expect(second.result.current.value.title).toBe("Still unsent when closed")
  await act(async () => second.result.current.flush())
  expect(readPageDocument(server).title).toBe("Still unsent when closed")
  second.unmount()
})

test("a retained Yjs copy remains editable when opening the server draft is temporarily unavailable", async () => {
  records.set("offline", {
    id: "offline", revision: 2, kind: "page", phase: "editable", scope: "page-collaboration:14", updatedAt: Date.now(),
    payload: { state: Y.encodeStateAsUpdate(server) },
  })
  const transport = jest.mocked(http.post).getMockImplementation()!
  jest.mocked(http.post).mockImplementation(async (url, body, options) => {
    if (url === "/api/page-drafts") throw new Error("Offline")
    return transport(url, body, options)
  })

  const editor = renderHook(() => usePageCollaboration(page))
  await waitFor(() => expect(editor.result.current.session).toBeDefined())
  expect(editor.result.current.error).toBeDefined()
  act(() => editor.result.current.change({ content: "Edited while offline" }))
  expect(editor.result.current.value.content).toBe("Edited while offline")

  jest.mocked(http.post).mockImplementation(transport)
  await act(async () => editor.result.current.flush())
  expect(readPageDocument(server).content).toBe("Edited while offline")
  expect(editor.result.current.error).toBeUndefined()
  editor.unmount()
})

test("disconnection removes remote cursors while preserving the local document", async () => {
  const editor = renderHook(() => usePageCollaboration(page))
  await waitFor(() => expect(editor.result.current.session).toBeDefined())
  const connection = Source.connections[0]
  const session = editor.result.current.session!
  const position = Y.createRelativePositionFromTypeIndex(session.document.getText("content"), 1)

  act(() => connection.onmessage!({ data: JSON.stringify({
    type: "cursor", clientId: 123, user: { id: 9, name: "Other editor" }, anchor: position, head: position,
  }) }))
  expect(session.awareness.getStates().get(123)?.user.name).toBe("Other editor")

  act(() => connection.onerror!())
  expect(session.awareness.getStates().has(123)).toBe(false)
  expect(editor.result.current.value.content).toBe("First\nSecond")
  editor.unmount()
})

test.each([[0, 12], [-1, 0]])("remote boundary positions tolerate omitted null JSON fields: association %i", async (association, expectedIndex) => {
  const editor = renderHook(() => usePageCollaboration(page))
  await waitFor(() => expect(editor.result.current.session).toBeDefined())
  const session = editor.result.current.session!
  const position = { tname: "content", assoc: association }

  act(() => Source.connections[0].onmessage!({ data: JSON.stringify({
    type: "cursor",
    clientId: 123,
    user: { id: 9, name: "Other editor" },
    anchor: position,
    head: position,
  }) }))

  const cursor = session.awareness.getStates().get(123)!.cursor
  expect(Y.createAbsolutePositionFromRelativePosition(cursor.anchor, session.document)!.index).toBe(expectedIndex)
  expect(Y.createAbsolutePositionFromRelativePosition(cursor.head, session.document)!.index).toBe(expectedIndex)
  editor.unmount()
})

test("removing an unuploaded banner also removes its local recovery bytes", async () => {
  const editor = renderHook(() => usePageCollaboration(page))
  await waitFor(() => expect(editor.result.current.session).toBeDefined())

  act(() => editor.result.current.change({
    bannerImage: { kind: "local", fileId: "banner", file: new File(["image"], "banner.png", { type: "image/png" }) },
  }))
  await waitFor(() => expect([...records.values()][0]?.payload.bannerImage?.kind).toBe("local"))

  act(() => editor.result.current.change({ bannerImage: null }))
  await waitFor(() => expect([...records.values()][0]?.payload.bannerImage).toBeUndefined())
  expect(editor.result.current.value.bannerImage).toBeNull()
  await act(async () => editor.result.current.flush())
  editor.unmount()
})
