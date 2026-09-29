import React from "react"
import { act, fireEvent, render, screen } from "@testing-library/react"
import { afterEach, beforeEach, expect, test } from "@jest/globals"
import { CommentComposer } from "./CommentComposer"
import { AccountDraft } from "@fider/services/browserDrafts"
import { DraftImage } from "@fider/services/draftImages"
import { editComment, submitComment } from "@fider/services/discussion"
import { analytics } from "@fider/services/analytics"

let mockNextIdentity = 0
let mockAccount: number | undefined
let mockPending: AccountDraft<any>[] = []
let mockLoaded = true
const mockDraftChange = jest.fn()
const mockComplete = jest.fn<Promise<boolean>, [unknown]>()
const mockReopen = jest.fn<Promise<void>, [unknown]>()
const mockReadUploads = jest.fn<Promise<DraftImage[]>, []>()

jest.mock("@fider/hooks/useAccountDraft", () => ({
  useAccountDraft: ({ initial }: any) => {
    const React = jest.requireActual("react")
    const [value, setValue] = React.useState(initial)
    const [restored, setRestored] = React.useState(0)
    const latest = React.useRef(value)
    const change = (update: any) => {
      latest.current = typeof update === "function" ? update(latest.current) : { ...latest.current, ...update }
      mockDraftChange(latest.current)
      setValue(latest.current)
    }
    return {
      value, status: "idle", loaded: mockLoaded, alternatives: [], pending: mockPending, restored, change,
      flush: async () => latest.current,
      seal: async () => {
        const attachments = await mockReadUploads()
        const id = `submission-${++mockNextIdentity}`
        return { payload: { ...latest.current, attachments, submissionId: id }, draft: { id, revision: 1 } }
      },
      reopen: async (next: any) => {
        change(next)
        setRestored((value: number) => value + 1)
        await mockReopen(next)
      },
      complete: mockComplete,
      reset: () => {
        change(initial)
        setRestored((value: number) => value + 1)
      },
    }
  },
}))

jest.mock("@fider/hooks/use-fider", () => ({
  useFider: () => ({ session: { tenant: { id: 1 }, isAuthenticated: mockAccount !== undefined, user: { id: mockAccount } } }),
}))
jest.mock("@fider/components/common/DraftPicker", () => ({ DraftPicker: () => null }))
jest.mock("@fider/services/discussion", () => ({ submitComment: jest.fn(), editComment: jest.fn() }))
jest.mock("@fider/services/analytics", () => ({ analytics: { event: jest.fn() } }))
jest.mock("./CommentAttachments", () => ({ CommentAttachments: () => <div data-testid="attachments" /> }))
jest.mock("@fider/components/common/form/Form", () => ({
  Form: ({ children, error }: any) => (
    <div>
      {error?.errors?.filter((item: any) => !item.field).map((item: any) => <p key={item.message}>{item.message}</p>)}
      {children}
    </div>
  ),
}))
jest.mock("@fider/components/auth/SignInModal", () => ({
  SignInModal: ({ isOpen }: any) => isOpen ? <div role="dialog">Sign in</div> : null,
}))
jest.mock("@fider/components/common/form/CommentEditor", () => ({
  CommentEditor: ({ initialValue, readOnly, onChange }: any) => (
    <textarea aria-label="Comment" defaultValue={initialValue} disabled={readOnly} onChange={event => onChange(event.target.value)} />
  ),
}))

const owner = { kind: "page" as const, id: 1, title: "Page", url: "/pages/page" }
const pending: AccountDraft<any> = {
  id: "original-operation",
  revision: 3,
  kind: "comment",
  phase: "pending",
  scope: "1:page:1:13",
  updatedAt: Date.now(),
  payload: { content: "Unconfirmed reply", attachments: [], parentId: 13 },
}

beforeEach(() => {
  jest.clearAllMocks()
  mockNextIdentity = 0
  mockAccount = 1
  mockPending = []
  mockLoaded = true
  mockReadUploads.mockResolvedValue([])
  mockComplete.mockResolvedValue(true)
  mockReopen.mockResolvedValue(undefined)
  jest.mocked(submitComment).mockImplementation(async (_owner, payload) => ({ ok: true, data: { id: 10, content: payload.content } as any }))
  jest.mocked(editComment).mockImplementation(async (_id, payload) => ({ ok: true, data: { id: 10, content: payload.content } as any }))
})

afterEach(() => jest.restoreAllMocks())

async function write(content: string) {
  fireEvent.change(screen.getByRole("textbox", { name: "Comment" }), { target: { value: content } })
  fireEvent.click(screen.getByRole("button", { name: "Submit comment" }))
  await act(async () => {})
}

test("ordinary edits have one draft owner and do not start delivery", () => {
  render(<CommentComposer owner={owner} images={false} onSaved={jest.fn()} />)
  const input = screen.getByRole("textbox", { name: "Comment" })
  fireEvent.change(input, { target: { value: "Still writing" } })

  expect(mockDraftChange).toHaveBeenCalledWith(expect.objectContaining({ content: "Still writing" }))
  expect(screen.getByRole("textbox", { name: "Comment" })).toBe(input)
  expect(submitComment).not.toHaveBeenCalled()
})

test("slow recovery cannot replace text typed after opening the composer", async () => {
  mockLoaded = false
  const view = render(<CommentComposer owner={owner} images={false} onSaved={jest.fn()} />)
  fireEvent.change(screen.getByRole("textbox", { name: "Comment" }), { target: { value: "Still writing" } })

  mockPending = [pending]
  mockLoaded = true
  view.rerender(<CommentComposer owner={owner} images={false} onSaved={jest.fn()} />)
  await act(async () => {})

  expect(screen.getByRole("textbox", { name: "Comment" })).toHaveValue("Still writing")
  expect(screen.getByRole("button", { name: "Submit comment" })).toBeEnabled()
})

test("writing another comment keeps the previous uncertain operation", async () => {
  mockPending = [pending]
  render(<CommentComposer owner={owner} images={false} onSaved={jest.fn()} />)
  await act(async () => {})

  fireEvent.click(screen.getByRole("button", { name: "Write another comment" }))
  await write("A separate comment")

  expect(mockComplete).toHaveBeenCalledWith({ id: "submission-1", revision: 1 })
  expect(mockComplete).not.toHaveBeenCalledWith({ id: pending.id, revision: pending.revision })
})

test("validation preserves editable content even when saving the rejection fails", async () => {
  jest.mocked(submitComment).mockResolvedValueOnce({ ok: false, status: 400, error: { errors: [{ field: "content", message: "Please change this comment." }] } })
  mockReopen.mockRejectedValueOnce(new Error("Storage unavailable"))
  jest.spyOn(console, "error").mockImplementation(() => {})
  const onSaved = jest.fn()
  render(<CommentComposer owner={owner} images={false} onSaved={onSaved} />)
  await write("Original comment")

  expect(screen.getByText("Please change this comment.")).toBeVisible()
  expect(screen.getByRole("textbox", { name: "Comment" })).toBeEnabled()
  await write("Corrected comment")

  expect(jest.mocked(submitComment).mock.calls.map(([, payload]) => payload.submissionId)).toEqual(["submission-1", "submission-2"])
  expect(onSaved).toHaveBeenCalledWith({ id: 10, content: "Corrected comment" })
  expect(screen.queryByText("Please change this comment.")).toBeNull()
})

test("an uncertain pending reply retries its original identity and parent", async () => {
  mockPending = [pending]
  jest.mocked(submitComment).mockRejectedValueOnce(new Error("Lost acknowledgement"))
  const onSaved = jest.fn()
  render(<CommentComposer owner={owner} parentId={13} images={false} onSaved={onSaved} />)
  await act(async () => {})

  const retry = screen.getByRole("button", { name: "Retry submission" })
  fireEvent.click(retry)
  await act(async () => {})
  expect(screen.getByRole("textbox", { name: "Comment" })).toHaveValue("Unconfirmed reply")
  fireEvent.click(retry)
  await act(async () => {})

  expect(submitComment).toHaveBeenCalledTimes(2)
  for (const [target, payload] of jest.mocked(submitComment).mock.calls) {
    expect(target).toEqual(owner)
    expect(payload).toEqual({ submissionId: pending.id, parentId: 13, content: "Unconfirmed reply", attachments: [] })
  }
  expect(onSaved).toHaveBeenCalledTimes(1)
  expect(mockComplete).toHaveBeenCalledWith({ id: pending.id, revision: pending.revision })
})

test.each([true, false, "unavailable"])("confirmation remains usable with completion result %s", async completion => {
  if (completion === "unavailable") {
    mockComplete.mockRejectedValue(new Error("Storage unavailable"))
    jest.spyOn(console, "error").mockImplementation(() => {})
  } else {
    mockComplete.mockResolvedValue(completion)
  }
  const onSaved = jest.fn()
  const onClose = jest.fn()
  render(<CommentComposer owner={owner} images={true} onSaved={onSaved} onClose={onClose} />)
  await write("Confirmed comment")

  expect(onSaved).toHaveBeenCalledWith({ id: 10, content: "Confirmed comment" })
  expect(onClose).toHaveBeenCalledTimes(1)
  expect(screen.getByRole("textbox", { name: "Comment" })).toHaveValue("")
  expect(screen.getAllByTestId("attachments")).toHaveLength(1)
  expect(analytics.event).toHaveBeenCalledTimes(completion === true ? 1 : 0)
})

test("a completed edit emits the update event", async () => {
  const comment = { id: 22, content: "Original", attachments: [] } as any
  render(<CommentComposer owner={owner} comment={comment} images={false} onSaved={jest.fn()} />)
  fireEvent.change(screen.getByRole("textbox", { name: "Comment" }), { target: { value: "Edited" } })
  fireEvent.click(screen.getByRole("button", { name: "Save changes" }))
  await act(async () => {})

  expect(editComment).toHaveBeenCalledWith(22, expect.objectContaining({ content: "Edited" }), expect.any(AbortSignal))
  expect(analytics.event).toHaveBeenCalledWith("comment", "update")
})

test("a removed composer publishes a completed result without closing another editor", async () => {
  let finish!: (result: any) => void
  jest.mocked(submitComment).mockImplementation(() => new Promise(resolve => { finish = resolve }))
  const onSaved = jest.fn()
  const onClose = jest.fn()
  const mounted = render(<CommentComposer owner={owner} images={false} onSaved={onSaved} onClose={onClose} />)
  await write("Still sending")
  mounted.unmount()
  await act(async () => finish({ ok: true, data: { id: 10, content: "Still sending" } }))

  expect(onSaved).toHaveBeenCalledTimes(1)
  expect(mockComplete).toHaveBeenCalledTimes(1)
  expect(onClose).not.toHaveBeenCalled()
})

test("an account switch during image preparation cannot start the old delivery", async () => {
  let finish!: (images: DraftImage[]) => void
  mockReadUploads.mockImplementation(() => new Promise(resolve => { finish = resolve }))
  const view = render(<CommentComposer owner={owner} images={true} onSaved={jest.fn()} />)
  await write("Prepared by the old editor")
  mockAccount = 2
  view.rerender(<CommentComposer owner={owner} images={true} onSaved={jest.fn()} />)
  await act(async () => finish([]))

  expect(submitComment).not.toHaveBeenCalled()
  expect(screen.getByRole("textbox", { name: "Comment" })).toBeEnabled()
})

test("a first authentication rejection preserves editable comment text", async () => {
  jest.mocked(submitComment).mockResolvedValueOnce({ ok: false, status: 401, error: { errors: [{ message: "Sign in to continue." }] } })
  render(<CommentComposer owner={owner} images={false} onSaved={jest.fn()} />)
  await write("Retained through sign in")
  expect(screen.getByRole("dialog")).toHaveTextContent("Sign in")

  expect(screen.getByRole("textbox", { name: "Comment" })).toBeEnabled()
  await write("Retained through sign in")
  const calls = jest.mocked(submitComment).mock.calls
  expect(calls[1][1].content).toBe(calls[0][1].content)
  expect(calls[1][1].submissionId).not.toBe(calls[0][1].submissionId)
})
