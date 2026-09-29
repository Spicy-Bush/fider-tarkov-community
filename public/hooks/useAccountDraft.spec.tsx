import { act, renderHook, waitFor } from "@testing-library/react"
import { accountDrafts, AccountDraft } from "@fider/services/browserDrafts"
import { DraftImage } from "@fider/services/draftImages"
import { useAccountDraft } from "./useAccountDraft"
import { useDraftSubmission } from "./useDraftSubmission"
import { Result } from "@fider/services/http"

jest.mock("./use-fider", () => ({
  useFider: () => ({ session: { tenant: { id: 1 }, user: { id: 7 }, isAuthenticated: true } }),
}))
jest.mock("@fider/services/browserDrafts", () => ({
  accountDrafts: {
    list: jest.fn(), save: jest.fn(), seal: jest.fn(), resume: jest.fn(), remove: jest.fn(),
    adoptAnonymous: jest.fn(), markUnsaved: jest.fn(),
  },
}))
jest.mock("@fider/services/postSubmission", () => {
  let id = 0
  return { newSubmissionID: () => `draft-${++id}` }
})

const initial = { title: "Published title", content: "Published body", attachments: [] as DraftImage[] }
const saved: AccountDraft<typeof initial> = {
  id: "existing",
  kind: "page",
  phase: "editable",
  scope: "page:14",
  revision: 3,
  payload: { ...initial, title: "", content: "Unfinished body" },
  updatedAt: 100,
}
const records = new Map<string, AccountDraft<any>>()

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>(done => { resolve = done })
  return { promise, resolve }
}

function editor() {
  return renderHook(() => useAccountDraft({ kind: "page", scope: "page:14", initial }))
}

beforeEach(() => {
  jest.clearAllMocks()
  records.clear()
  jest.mocked(accountDrafts.list).mockResolvedValue([])
  jest.mocked(accountDrafts.adoptAnonymous).mockResolvedValue()
  jest.mocked(accountDrafts.resume).mockResolvedValue(saved)
  jest.mocked(accountDrafts.save).mockImplementation(async (_, value) => {
    const draft = { ...value, phase: "editable" as const, revision: value.revision + 1, updatedAt: 100 }
    records.set(draft.id, draft)
    return { accepted: true, draft }
  })
  jest.mocked(accountDrafts.seal).mockImplementation(async (_, receipt) => ({
    accepted: true,
    draft: { ...records.get(receipt.id)!, phase: "pending", revision: receipt.revision + 1 },
  }))
  jest.mocked(accountDrafts.remove).mockResolvedValue(true)
})

test("restores empty saved fields and exposes pending work separately", async () => {
  const pending = { ...saved, id: "pending", phase: "pending" as const }
  jest.mocked(accountDrafts.list).mockResolvedValue([pending, saved])
  const draft = editor()
  await waitFor(() => expect(draft.result.current.loaded).toBe(true))

  expect(draft.result.current.value).toEqual(saved.payload)
  expect(draft.result.current.pending).toEqual([pending])
  expect(accountDrafts.save).not.toHaveBeenCalled()
  expect(accountDrafts.adoptAnonymous).toHaveBeenCalledWith("1:7", "page", "page:14")
})

test("typing during initial loading preserves both the typing and the saved alternative", async () => {
  const reply = deferred<AccountDraft[]>()
  jest.mocked(accountDrafts.list).mockReturnValueOnce(reply.promise)
  const draft = editor()
  act(() => draft.result.current.change({ content: "New typing" }))
  await act(async () => { reply.resolve([saved]) })
  await act(async () => { await draft.result.current.flush() })

  expect(draft.result.current.value.content).toBe("New typing")
  expect(draft.result.current.alternatives).toEqual([saved])
  expect(accountDrafts.save).toHaveBeenCalledWith("1:7", expect.objectContaining({
    payload: expect.objectContaining({ content: "New typing" }),
  }), undefined)
})

test("a delayed write retains newer text and image changes in the next revision", async () => {
  const reply = deferred<Awaited<ReturnType<typeof accountDrafts.save>>>()
  jest.mocked(accountDrafts.save).mockReturnValueOnce(reply.promise)
  const draft = editor()
  await waitFor(() => expect(draft.result.current.loaded).toBe(true))
  const file = new File(["image bytes"], "image.png", { type: "image/png" })
  const image: DraftImage = { kind: "local", fileId: "image", file }
  act(() => draft.result.current.change({ content: "First", attachments: [image] }))
  let write!: Promise<typeof initial>
  act(() => { write = draft.result.current.flush() })
  await waitFor(() => expect(accountDrafts.save).toHaveBeenCalledTimes(1))

  act(() => draft.result.current.change({ content: "Second", attachments: [] }))
  const first = jest.mocked(accountDrafts.save).mock.calls[0][1]
  await act(async () => {
    reply.resolve({ accepted: true, draft: { ...first, phase: "editable", revision: 1, updatedAt: 100 } })
    await write
  })

  expect(first.payload.attachments).toEqual([image])
  expect(accountDrafts.save).toHaveBeenLastCalledWith("1:7", expect.objectContaining({
    id: first.id,
    revision: 1,
    payload: { ...initial, content: "Second", attachments: [] },
  }), undefined)
  expect(draft.result.current.status).toBe("saved")
})

test("concurrent editing forks local work without overwriting the other tab", async () => {
  jest.mocked(accountDrafts.list).mockResolvedValue([saved])
  const remote = { ...saved, revision: 4, payload: { ...initial, content: "Other tab" } }
  jest.mocked(accountDrafts.save).mockResolvedValueOnce({ accepted: false, draft: remote })
  const draft = editor()
  await waitFor(() => expect(draft.result.current.loaded).toBe(true))
  await act(async () => {
    draft.result.current.change({ content: "This tab" })
    await draft.result.current.flush()
  })

  const writes = jest.mocked(accountDrafts.save).mock.calls
  expect(writes[0][1].id).toBe(saved.id)
  expect(writes[1][1].id).not.toBe(saved.id)
  expect(writes[1][1].revision).toBe(0)
  expect(draft.result.current.value.content).toBe("This tab")
  expect(draft.result.current.alternatives).toEqual([remote])
})

test("storage failure retains text, permits publication, and reuses its submission identity", async () => {
  const draft = editor()
  await waitFor(() => expect(draft.result.current.loaded).toBe(true))
  jest.mocked(accountDrafts.save).mockRejectedValue(new DOMException("Full", "QuotaExceededError"))
  act(() => draft.result.current.change({ content: "Publish this" }))
  let submission!: Awaited<ReturnType<typeof draft.result.current.seal>>
  await act(async () => { submission = await draft.result.current.seal() })

  expect(submission.payload.content).toBe("Publish this")
  expect(submission.payload.submissionId).toBe(submission.draft.id)
  expect(draft.result.current.status).toBe("error")
  await act(async () => { expect(await draft.result.current.seal()).toEqual(submission) })
  expect(accountDrafts.markUnsaved).toHaveBeenCalledWith(submission.draft.id, true)

  await act(async () => { await draft.result.current.complete(submission.draft) })
  expect(accountDrafts.markUnsaved).toHaveBeenLastCalledWith(submission.draft.id, false)
})

test("a failed write recovers using the retained value and original draft identity", async () => {
  const draft = editor()
  await waitFor(() => expect(draft.result.current.loaded).toBe(true))
  jest.mocked(accountDrafts.save).mockRejectedValueOnce(new Error("Unavailable"))
  await act(async () => {
    draft.result.current.change({ content: "Keep this" })
    await expect(draft.result.current.flush()).rejects.toThrow("Unavailable")
  })
  await act(async () => { await draft.result.current.flush() })

  const writes = jest.mocked(accountDrafts.save).mock.calls
  expect(writes[1][1]).toEqual(writes[0][1])
  expect(draft.result.current.status).toBe("saved")
  expect(draft.result.current.value.content).toBe("Keep this")
})

test("unmounting does not discard edits queued behind an unfinished write", async () => {
  const reply = deferred<Awaited<ReturnType<typeof accountDrafts.save>>>()
  jest.mocked(accountDrafts.save).mockReturnValueOnce(reply.promise)
  const draft = editor()
  await waitFor(() => expect(draft.result.current.loaded).toBe(true))
  act(() => draft.result.current.change({ content: "First" }))
  let write!: Promise<typeof initial>
  act(() => { write = draft.result.current.flush() })
  await waitFor(() => expect(accountDrafts.save).toHaveBeenCalledTimes(1))
  act(() => draft.result.current.change({ content: "Before leaving" }))
  draft.unmount()

  const first = jest.mocked(accountDrafts.save).mock.calls[0][1]
  reply.resolve({ accepted: true, draft: { ...first, phase: "editable", revision: 1, updatedAt: 100 } })
  await write
  expect(accountDrafts.save).toHaveBeenLastCalledWith("1:7", expect.objectContaining({
    payload: expect.objectContaining({ content: "Before leaving" }),
  }), undefined)
})

test("resuming a different draft does not replace typing made during the read", async () => {
  const draft = editor()
  await waitFor(() => expect(draft.result.current.loaded).toBe(true))
  const reply = deferred<AccountDraft<typeof initial> | null>()
  jest.mocked(accountDrafts.resume).mockReturnValueOnce(reply.promise)
  let reading!: Promise<void>
  act(() => { reading = draft.result.current.resume(saved) })
  await waitFor(() => expect(accountDrafts.resume).toHaveBeenCalledTimes(1))
  act(() => draft.result.current.change({ content: "Typed while loading" }))
  await act(async () => { reply.resolve(saved); await reading })

  expect(draft.result.current.value.content).toBe("Typed while loading")
})

test("rejected work replaces its pending record only after the editable replacement is saved", async () => {
  const draft = editor()
  await waitFor(() => expect(draft.result.current.loaded).toBe(true))
  const receipt = { id: "pending", revision: 3 }
  const submission = { ...initial, content: "Rejected", submissionId: receipt.id }
  await act(async () => { await draft.result.current.reopen(submission, receipt) })

  expect(accountDrafts.save).toHaveBeenCalledWith("1:7", expect.objectContaining({
    revision: 0,
    payload: expect.objectContaining({ content: "Rejected" }),
  }), receipt)
  expect(draft.result.current.value.content).toBe("Rejected")
  expect(draft.result.current.value).not.toHaveProperty("submissionId")
})

test("an initial read failure can recover without overwriting subsequent typing", async () => {
  const error = jest.spyOn(console, "error").mockImplementation(() => {})
  jest.mocked(accountDrafts.list).mockRejectedValueOnce(new Error("Database unavailable")).mockResolvedValue([saved])
  const draft = editor()
  await waitFor(() => expect(draft.result.current.status).toBe("error"))
  await act(async () => {
    draft.result.current.change({ content: "Typed during failure" })
    await draft.result.current.flush()
  })

  expect(draft.result.current.status).toBe("saved")
  expect(draft.result.current.value.content).toBe("Typed during failure")
  expect(draft.result.current.alternatives).toEqual([saved])
  error.mockRestore()
})

test("typing while rejected work is reopening survives the delayed write", async () => {
  const reply = deferred<Awaited<ReturnType<typeof accountDrafts.save>>>()
  jest.mocked(accountDrafts.save).mockReturnValueOnce(reply.promise)
  const draft = editor()
  await waitFor(() => expect(draft.result.current.loaded).toBe(true))
  act(() => draft.result.current.change({ content: "Before rejection" }))
  act(() => { void draft.result.current.flush() })
  await waitFor(() => expect(accountDrafts.save).toHaveBeenCalledTimes(1))

  let reopening!: Promise<typeof initial>
  act(() => { reopening = draft.result.current.reopen({ ...initial, content: "Rejected content" }) })
  act(() => draft.result.current.change({ content: "New typing" }))
  const first = jest.mocked(accountDrafts.save).mock.calls[0][1]
  await act(async () => {
    reply.resolve({ accepted: true, draft: { ...first, phase: "editable", revision: 1, updatedAt: 100 } })
    await reopening
  })

  expect(draft.result.current.value.content).toBe("New typing")
  expect(accountDrafts.save).toHaveBeenLastCalledWith("1:7", expect.objectContaining({
    payload: expect.objectContaining({ content: "New typing" }),
  }), undefined)
})

test("a conflicting submission stays pending while this tab saves its separate edits", async () => {
  const pending = { ...saved, phase: "pending" as const, revision: 4 }
  jest.mocked(accountDrafts.list).mockResolvedValue([saved])
  jest.mocked(accountDrafts.save).mockResolvedValueOnce({ accepted: false, draft: pending })
  const draft = editor()
  await waitFor(() => expect(draft.result.current.loaded).toBe(true))

  await act(async () => {
    draft.result.current.change({ content: "Independent local changes" })
    await draft.result.current.flush()
  })

  expect(draft.result.current.value.content).toBe("Independent local changes")
  expect(draft.result.current.pending).toEqual([pending])
  expect(draft.result.current.alternatives).toEqual([])
  expect(accountDrafts.save).toHaveBeenLastCalledWith("1:7", expect.objectContaining({
    revision: 0,
    payload: expect.objectContaining({ content: "Independent local changes" }),
  }), undefined)
})

test("a failed draft selection stays visible, preserves current work, and can be retried", async () => {
  const draft = editor()
  await waitFor(() => expect(draft.result.current.loaded).toBe(true))
  jest.mocked(accountDrafts.resume).mockRejectedValueOnce(new Error("Read unavailable"))

  await act(async () => {
    draft.result.current.change({ content: "Keep current text" })
    await expect(draft.result.current.resume(saved)).rejects.toThrow("Read unavailable")
  })
  expect(draft.result.current.value.content).toBe("Keep current text")
  expect(draft.result.current.status).toBe("error")
  expect(draft.result.current.error).toContain("retry the selection")

  await act(async () => { await draft.result.current.resume(saved) })
  expect(draft.result.current.value).toEqual(saved.payload)
  expect(draft.result.current.status).toBe("saved")
})

test("selection rechecks a draft that another tab has already submitted", async () => {
  const pending = { ...saved, phase: "pending" as const, revision: 4 }
  jest.mocked(accountDrafts.resume).mockResolvedValueOnce(pending)
  const draft = editor()
  await waitFor(() => expect(draft.result.current.loaded).toBe(true))
  await act(async () => { await draft.result.current.resume(saved) })

  expect(draft.result.current.value).toEqual(initial)
  expect(draft.result.current.pending).toEqual([pending])
  expect(draft.result.current.alternatives).toEqual([])
})

test("sealing a restored draft uses the store's receipt without saving it again", async () => {
  const pending = { ...saved, phase: "pending" as const, revision: saved.revision + 1 }
  jest.mocked(accountDrafts.list).mockResolvedValue([saved])
  jest.mocked(accountDrafts.seal).mockResolvedValue({ accepted: true, draft: pending })
  const draft = editor()
  await waitFor(() => expect(draft.result.current.loaded).toBe(true))

  await act(async () => {
    const result = await draft.result.current.seal()
    expect(result.draft).toEqual({ id: saved.id, revision: 4 })
    expect(result.payload).toEqual({ ...saved.payload, submissionId: saved.id })
  })

  expect(accountDrafts.save).not.toHaveBeenCalled()
  expect(accountDrafts.seal).toHaveBeenCalledTimes(1)
  expect(accountDrafts.seal).toHaveBeenCalledWith("1:7", { id: saved.id, revision: 3 })
})

test("an edit arriving before sealing is preserved while this editor submits its own value separately", async () => {
  const changed = { ...saved, revision: 4, payload: { ...initial, content: "Other tab changed this" } }
  jest.mocked(accountDrafts.list).mockResolvedValue([saved])
  jest.mocked(accountDrafts.seal).mockResolvedValueOnce({ accepted: false, draft: changed })
  const draft = editor()
  await waitFor(() => expect(draft.result.current.loaded).toBe(true))

  await act(async () => {
    const result = await draft.result.current.seal()
    expect(result.draft.id).not.toBe(saved.id)
    expect(result.payload.content).toBe(saved.payload.content)
  })
  expect(draft.result.current.alternatives).toEqual([changed])
  expect(draft.result.current.pending).toHaveLength(1)
})

test("an unsaved changed value gets a fresh stable submission ID after storage failure", async () => {
  jest.mocked(accountDrafts.list).mockResolvedValue([saved])
  const draft = editor()
  await waitFor(() => expect(draft.result.current.loaded).toBe(true))
  jest.mocked(accountDrafts.save).mockRejectedValue(new Error("Full"))

  await act(async () => {
    draft.result.current.change({ content: "New unsaved intent" })
    const first = await draft.result.current.seal()
    const retry = await draft.result.current.seal()
    expect(first.draft.id).not.toBe(saved.id)
    expect(first.payload.content).toBe("New unsaved intent")
    expect(retry).toEqual(first)
  })
  expect(accountDrafts.seal).not.toHaveBeenCalled()
})

test("switching drafts retains the previous selection and excludes the active record", async () => {
  const other = { ...saved, id: "other", payload: { ...initial, title: "Other draft" } }
  jest.mocked(accountDrafts.list).mockResolvedValue([saved, other])
  jest.mocked(accountDrafts.resume).mockResolvedValue(other)
  const draft = editor()
  await waitFor(() => expect(draft.result.current.loaded).toBe(true))
  await act(async () => { await draft.result.current.resume(other) })

  expect(draft.result.current.value.title).toBe("Other draft")
  expect(draft.result.current.alternatives).toEqual([saved])
})

test("record events update only that record and never replace the editor buffer", async () => {
  const pending = { ...saved, id: "remote", phase: "pending" as const }
  const draft = editor()
  await waitFor(() => expect(draft.result.current.loaded).toBe(true))
  jest.mocked(accountDrafts.resume).mockResolvedValueOnce(pending).mockResolvedValueOnce(null)

  const changed = (detail: { account: string; id: string; scope?: string }) =>
    window.dispatchEvent(new CustomEvent("account-drafts-changed", { detail }))
  await act(async () => {
    draft.result.current.change({ content: "This editor owns this text" })
    changed({ account: "1:8", id: "remote" })
    changed({ account: "1:7", id: "remote", scope: "other" })
    changed({ account: "1:7", id: "remote", scope: "page:14" })
  })
  expect(accountDrafts.resume).toHaveBeenCalledTimes(1)
  expect(accountDrafts.list).toHaveBeenCalledTimes(1)
  expect(draft.result.current.value.content).toBe("This editor owns this text")
  expect(draft.result.current.pending).toEqual([pending])

  await act(async () => { changed({ account: "1:7", id: "remote" }) })
  expect(draft.result.current.pending).toEqual([])
  expect(accountDrafts.list).toHaveBeenCalledTimes(1)
})

test("an own autosave updates the record cache without another database read", async () => {
  const original = jest.mocked(accountDrafts.save).getMockImplementation()!
  jest.mocked(accountDrafts.save).mockImplementation(async (...args) => {
    const result = await original(...args)
    window.dispatchEvent(new CustomEvent("account-drafts-changed", {
      detail: { account: args[0], id: args[1].id, scope: args[1].scope },
    }))
    return result
  })
  const draft = editor()
  await waitFor(() => expect(draft.result.current.loaded).toBe(true))
  await act(async () => {
    draft.result.current.change({ content: "Autosave" })
    await draft.result.current.flush()
  })

  expect(accountDrafts.resume).not.toHaveBeenCalled()
  expect(accountDrafts.list).toHaveBeenCalledTimes(1)
})

test("a change arriving during initialization is applied after the initial snapshot", async () => {
  const initialRead = deferred<AccountDraft[]>()
  const pending = { ...saved, phase: "pending" as const }
  jest.mocked(accountDrafts.list).mockReturnValueOnce(initialRead.promise)
  jest.mocked(accountDrafts.resume).mockResolvedValueOnce(pending)
  const draft = editor()
  act(() => window.dispatchEvent(new CustomEvent("account-drafts-changed", {
    detail: { account: "1:7", id: saved.id, scope: saved.scope },
  })))
  expect(accountDrafts.resume).not.toHaveBeenCalled()

  await act(async () => { initialRead.resolve([]) })
  expect(draft.result.current.pending).toEqual([pending])
  expect(draft.result.current.value).toEqual(initial)
})

test("an older record read cannot restore a submission after a newer completion", async () => {
  const firstRead = deferred<AccountDraft<typeof initial> | null>()
  jest.mocked(accountDrafts.resume).mockReturnValueOnce(firstRead.promise).mockResolvedValueOnce(null)
  const draft = editor()
  await waitFor(() => expect(draft.result.current.loaded).toBe(true))
  const changed = () => window.dispatchEvent(new CustomEvent("account-drafts-changed", {
    detail: { account: "1:7", id: saved.id, scope: saved.scope },
  }))

  await act(async () => { changed() })
  await act(async () => { changed() })
  await act(async () => { firstRead.resolve({ ...saved, phase: "pending" }) })

  expect(draft.result.current.pending).toEqual([])
  expect(accountDrafts.list).toHaveBeenCalledTimes(1)
})

test("failed record refresh can reload the list without adopting another draft over the current editor", async () => {
  const draft = editor()
  await waitFor(() => expect(draft.result.current.loaded).toBe(true))
  jest.mocked(accountDrafts.resume).mockRejectedValueOnce(new Error("Read failed"))
  jest.mocked(accountDrafts.list).mockResolvedValueOnce([saved])
  await act(async () => window.dispatchEvent(new CustomEvent("account-drafts-changed", {
    detail: { account: "1:7", id: saved.id, scope: saved.scope },
  })))
  expect(draft.result.current.error).toContain("could not be refreshed")

  await act(async () => { await draft.result.current.flush() })
  expect(draft.result.current.alternatives).toEqual([saved])
  expect(draft.result.current.value).toEqual(initial)
  expect(draft.result.current.error).toBeUndefined()
})

test("expiry clears saved choices without losing typed work or restoring an older read", async () => {
  const pending = { ...saved, id: "pending", phase: "pending" as const }
  const delayed = deferred<AccountDraft<typeof initial> | null>()
  jest.mocked(accountDrafts.list).mockResolvedValue([saved, pending])
  jest.mocked(accountDrafts.resume).mockReturnValueOnce(delayed.promise)
  const draft = editor()
  await waitFor(() => expect(draft.result.current.loaded).toBe(true))

  await act(async () => window.dispatchEvent(new CustomEvent("account-drafts-changed", {
    detail: { account: "1:7", id: pending.id, scope: pending.scope },
  })))
  act(() => {
    draft.result.current.change({ content: "Keep this unfinished text" })
    window.dispatchEvent(new CustomEvent("account-drafts-changed", {
      detail: { account: "1:7", expired: true },
    }))
  })
  await act(async () => { delayed.resolve(pending) })

  expect(draft.result.current.pending).toEqual([])
  expect(draft.result.current.alternatives).toEqual([])
  expect(draft.result.current.value.content).toBe("Keep this unfinished text")
  expect(accountDrafts.list).toHaveBeenCalledTimes(1)
})

test("a missing saved choice is removed from the picker and preserves the active text", async () => {
  const missing = { ...saved, id: "missing" }
  jest.mocked(accountDrafts.list).mockResolvedValue([saved, missing])
  jest.mocked(accountDrafts.resume).mockResolvedValueOnce(null)
  const draft = editor()
  await waitFor(() => expect(draft.result.current.loaded).toBe(true))
  await act(async () => { await draft.result.current.resume(missing) })

  expect(draft.result.current.alternatives).toEqual([])
  expect(draft.result.current.value).toEqual(saved.payload)
  expect(draft.result.current.error).toContain("expired or was removed")
})


test("rejected delivery reopens the real editor and corrected work gets a new submission identity", async () => {
  const send = jest.fn<Promise<Result<{ id: number }>>, any[]>()
    .mockResolvedValueOnce({ ok: false, status: 400, error: { errors: [{ message: "Correct the content." }] } })
    .mockResolvedValueOnce({ ok: true, data: { id: 14 } })
  const onSaved = jest.fn()
  const view = renderHook(() => {
    const draft = useAccountDraft({ kind: "comment", scope: "comment:new", initial })
    const submission = useDraftSubmission({ editor: draft, send, onSaved, onUnauthorized: jest.fn() })
    return { draft, submission }
  })
  await waitFor(() => expect(view.result.current.draft.loaded).toBe(true))

  await act(async () => { await view.result.current.submission.submit() })
  expect(view.result.current.submission.state.phase).toBe("idle")
  expect(view.result.current.draft.value).toMatchObject(initial)
  expect(onSaved).not.toHaveBeenCalled()

  act(() => view.result.current.draft.change({ content: "Corrected content" }))
  await act(async () => { await view.result.current.submission.submit() })

  expect(send.mock.calls[1][0].content).toBe("Corrected content")
  expect(send.mock.calls[1][0].submissionId).not.toBe(send.mock.calls[0][0].submissionId)
  expect(onSaved).toHaveBeenCalledWith({ id: 14 }, { firstCompletion: true, mounted: true })
})

test("a delayed saved-draft selection cannot replace an active submission", async () => {
  const response = deferred<Result<{ id: number }>>()
  const send = jest.fn(() => response.promise)
  const view = renderHook(() => {
    const draft = useAccountDraft({ kind: "comment", scope: "comment:new", initial })
    const submission = useDraftSubmission({ editor: draft, send, onSaved: jest.fn(), onUnauthorized: jest.fn() })
    return { draft, submission }
  })
  await waitFor(() => expect(view.result.current.draft.loaded).toBe(true))

  let completion!: Promise<void>
  act(() => { completion = view.result.current.submission.submit() })
  await waitFor(() => expect(send).toHaveBeenCalledTimes(1))
  const original = view.result.current.submission.state

  act(() => view.result.current.submission.select({ ...saved, phase: "pending" }))
  expect(view.result.current.submission.state).toBe(original)
  await act(async () => { await view.result.current.submission.submit() })
  expect(send).toHaveBeenCalledTimes(1)

  await act(async () => {
    response.resolve({ ok: true, data: { id: 18 } })
    await completion
  })
})

test.each([false, true])("a rejection reopens work only when no attempt is uncertain (%s)", async (unconfirmed) => {
  const send = jest.fn()
    .mockResolvedValueOnce({ ok: false, status: 403, unconfirmed, error: { errors: [{ message: "You are muted." }] } })
    .mockResolvedValueOnce({ ok: false, status: 401, unconfirmed: false, error: { errors: [{ message: "Sign in again." }] } })
    .mockResolvedValue({ ok: true, unconfirmed: false, data: { id: 14 } })
  const onSaved = jest.fn()
  const view = renderHook(() => {
    const draft = useAccountDraft({ kind: "comment", scope: "comment:new", initial })
    const submission = useDraftSubmission({ editor: draft, send, onSaved, onUnauthorized: jest.fn() })
    return { draft, submission }
  })
  await waitFor(() => expect(view.result.current.draft.loaded).toBe(true))

  await act(async () => { await view.result.current.submission.submit() })
  expect(view.result.current.submission.state.phase).toBe(unconfirmed ? "unconfirmed" : "idle")
  expect(view.result.current.draft.value).toMatchObject(initial)
  expect(view.result.current.submission.error?.errors?.[0].message).toBe("You are muted.")
  const original = send.mock.calls[0][0]

  await act(async () => { await view.result.current.submission.submit() })
  await act(async () => { await view.result.current.submission.submit() })
  if (unconfirmed) {
    expect(send.mock.calls[1][0]).toEqual(original)
    expect(send.mock.calls[2][0]).toEqual(original)
  } else {
    expect(send.mock.calls[1][0].submissionId).not.toBe(original.submissionId)
  }
  expect(onSaved).toHaveBeenCalledTimes(1)
})
