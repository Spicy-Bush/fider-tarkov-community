import React from "react"
import { act, fireEvent, render, screen } from "@testing-library/react"
import { afterEach, beforeEach, expect, test } from "@jest/globals"
import { CommentComposer } from "./CommentComposer"
import { CommentDraft, SavedComment, commentDrafts } from "@fider/services/commentDrafts"
import { submitComment } from "@fider/services/discussion"

let mockNextIdentity = 0

jest.mock("@fider/services/postSubmission", () => ({
  newSubmissionID: () => `submission-${++mockNextIdentity}`,
}))

jest.mock("@fider/hooks", () => ({
  useFider: () => ({ session: { tenant: { id: 1 } } }),
}))

jest.mock("@fider/services", () => ({ classSet: () => "" }))
jest.mock("@fider/services/discussion", () => ({ submitComment: jest.fn(), editComment: jest.fn() }))
jest.mock("@fider/services/commentDrafts", () => ({
  commentDrafts: {
    load: jest.fn(),
    recover: jest.fn(),
    subscribe: jest.fn(() => () => {}),
    prepare: jest.fn(),
    save: jest.fn(),
    complete: jest.fn(),
    rejected: jest.fn(),
  },
}))
jest.mock("./CommentAttachments", () => ({ CommentAttachments: () => <div data-testid="attachments" /> }))

jest.mock("@fider/components", () => ({
  Button: jest.requireActual("@fider/components/common/Button").Button,
  DisplayError: jest.requireActual("@fider/components/common/form/DisplayError").DisplayError,
  Form: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  SignInModal: () => null,
  CommentEditor: ({ initialValue, readOnly, onChange }: any) => (
    <textarea
      aria-label="Comment"
      defaultValue={initialValue}
      disabled={readOnly}
      onChange={(event) => onChange(event.target.value)}
    />
  ),
}))

const owner = { kind: "page" as const, id: 1, title: "Page", url: "/pages/page" }
const original: CommentDraft = {
  scope: "1:page:1:root",
  submissionId: "restored-operation",
  content: "Original draft",
  attachments: [],
  state: "draft",
  updatedAt: 1,
}

beforeEach(() => {
  jest.clearAllMocks()
  jest.useFakeTimers()
  mockNextIdentity = 0

  jest.mocked(commentDrafts.load).mockResolvedValue([{ ...original }])
  jest.mocked(commentDrafts.recover).mockReturnValue([])
  jest.mocked(commentDrafts.prepare).mockImplementation(async (draft) => ({
    submissionId: draft.submissionId,
    content: draft.content,
    parentId: draft.parentId,
    attachments: [],
  }))
  jest.mocked(commentDrafts.save).mockImplementation(async (draft) => draft)
  jest.mocked(commentDrafts.complete).mockResolvedValue(undefined)
  jest.mocked(commentDrafts.rejected).mockImplementation(async (draft) => ({ ...draft, state: "draft" }))
  jest.mocked(submitComment).mockImplementation(async (_owner, submission) => ({
    ok: true,
    data: { id: 10, content: submission.content } as any,
  }))
})

test("a rejection recovers a receipt completed by another tab", async () => {
  jest.mocked(commentDrafts.load).mockResolvedValue([])
  jest.mocked(submitComment).mockResolvedValue({ ok: false, status: 403, error: {} })
  jest.mocked(commentDrafts.rejected).mockImplementation(async (draft) => ({
    scope: draft.scope,
    submissionId: draft.submissionId,
    state: "completed",
    updatedAt: 2,
    comment: { id: 9, content: draft.content } as any,
  }))

  const onSaved = jest.fn()
  render(<CommentComposer owner={owner} images={false} onSaved={onSaved} />)
  await act(async () => {})

  fireEvent.change(screen.getByRole("textbox", { name: "Comment" }), { target: { value: "Completed in another tab" } })
  fireEvent.click(screen.getByRole("button", { name: "Submit comment" }))
  await act(async () => {})

  expect(onSaved).toHaveBeenCalledWith({ id: 9, content: "Completed in another tab" })
  expect(screen.getByRole("textbox", { name: "Comment" })).toHaveValue("")
})

test("content validation is visible and the rejected draft can be corrected", async () => {
  jest.mocked(submitComment).mockResolvedValueOnce({
    ok: false,
    status: 400,
    error: { errors: [{ field: "content", message: "Please change this comment." }] },
  })

  const onSaved = jest.fn()
  render(<CommentComposer owner={owner} images={false} onSaved={onSaved} />)
  await act(async () => {})

  fireEvent.click(screen.getByRole("button", { name: "Submit comment" }))
  await act(async () => {})

  expect(screen.getByText("Please change this comment.")).toBeVisible()
  expect(screen.getByRole("textbox", { name: "Comment" })).toHaveValue(original.content)

  fireEvent.change(screen.getByRole("textbox", { name: "Comment" }), { target: { value: "Corrected comment" } })
  fireEvent.click(screen.getByRole("button", { name: "Submit comment" }))
  await act(async () => {})

  expect(onSaved).toHaveBeenCalledWith({ id: 10, content: "Corrected comment" })
  expect(screen.queryByText("Please change this comment.")).toBeNull()
})

test("editing after rejection forks the operation even if another tab completes it later", async () => {
  jest.mocked(commentDrafts.load).mockResolvedValue([])
  jest.mocked(submitComment).mockResolvedValueOnce({ ok: false, status: 403, error: {} })

  const onSaved = jest.fn()
  render(<CommentComposer owner={owner} images={false} onSaved={onSaved} />)
  await act(async () => {})

  const editor = screen.getByRole("textbox", { name: "Comment" })
  fireEvent.change(editor, { target: { value: "Original comment" } })
  fireEvent.click(screen.getByRole("button", { name: "Submit comment" }))
  await act(async () => {})

  const originalSubmission = jest.mocked(submitComment).mock.calls[0][1]
  jest.mocked(commentDrafts.save).mockImplementation(async (draft) => {
    if (draft.submissionId === originalSubmission.submissionId) {
      return {
        scope: draft.scope,
        submissionId: draft.submissionId,
        state: "completed",
        updatedAt: 2,
        comment: { id: 9, content: "Original comment" } as any,
      }
    }

    return draft
  })

  fireEvent.change(editor, { target: { value: "New intent after rejection" } })
  expect(screen.getByRole("textbox", { name: "Comment" })).toBe(editor)

  fireEvent.click(screen.getByRole("button", { name: "Submit comment" }))
  await act(async () => {})

  expect(submitComment).toHaveBeenCalledTimes(2)
  expect(jest.mocked(submitComment).mock.calls[1][1].submissionId).not.toBe(originalSubmission.submissionId)
  expect(onSaved).toHaveBeenCalledWith({ id: 10, content: "New intent after rejection" })
})

afterEach(() => {
  jest.useRealTimers()
})

test("changing a restored draft keeps its editor but gives the changed intent its own submission", async () => {
  const onSaved = jest.fn()
  render(<CommentComposer owner={owner} images={false} onSaved={onSaved} />)
  await act(async () => {})

  const editor = screen.getByRole("textbox", { name: "Comment" })
  fireEvent.change(editor, { target: { value: "Independent second-tab draft" } })

  expect(screen.getByRole("textbox", { name: "Comment" })).toBe(editor)

  const receipt: SavedComment = {
    scope: original.scope,
    submissionId: original.submissionId,
    state: "completed",
    updatedAt: 2,
    comment: { id: 9, content: "First-tab comment" } as any,
  }
  jest.mocked(commentDrafts.save).mockImplementation(async (draft) => {
    return draft.submissionId === original.submissionId ? receipt : draft
  })

  fireEvent.click(screen.getByRole("button", { name: "Submit comment" }))
  await act(async () => {})

  expect(submitComment).toHaveBeenCalledTimes(1)
  expect(jest.mocked(submitComment).mock.calls[0][1]).toMatchObject({
    content: "Independent second-tab draft",
    submissionId: "submission-1",
  })
  expect(onSaved).toHaveBeenCalledWith({ id: 10, content: "Independent second-tab draft" })
  expect(screen.getByRole("textbox", { name: "Comment" })).toHaveValue("")
})

test("the last edit reaches draft storage before the composer is removed", async () => {
  const view = render(<CommentComposer owner={owner} images={false} onSaved={() => {}} />)
  await act(async () => {})

  fireEvent.change(screen.getByRole("textbox", { name: "Comment" }), { target: { value: "Last keystroke" } })
  expect(commentDrafts.save).toHaveBeenCalledWith(
    expect.objectContaining({ content: "Last keystroke", submissionId: "submission-1" }),
    original
  )
  view.unmount()
  await act(async () => {})
})

test("a mounted composer publishes its result and closes", async () => {
  const onSaved = jest.fn()
  const onClose = jest.fn()
  render(<CommentComposer owner={owner} images={false} onSaved={onSaved} onClose={onClose} />)
  await act(async () => {})

  fireEvent.click(screen.getByRole("button", { name: "Submit comment" }))
  await act(async () => {})

  expect(onSaved).toHaveBeenCalledWith({ id: 10, content: original.content })
  expect(onClose).toHaveBeenCalledTimes(1)
})

test("a removed composer publishes its result without closing a replacement editor", async () => {
  let complete!: (result: Awaited<ReturnType<typeof submitComment>>) => void
  jest.mocked(submitComment).mockReturnValueOnce(new Promise((resolve) => {
    complete = resolve
  }))

  const onSaved = jest.fn()
  const onClose = jest.fn()
  const previous = render(<CommentComposer owner={owner} images={false} onSaved={onSaved} onClose={onClose} />)
  await act(async () => {})

  fireEvent.click(screen.getByRole("button", { name: "Submit comment" }))
  await act(async () => {})
  previous.unmount()

  jest.mocked(commentDrafts.load).mockResolvedValue([])
  render(<CommentComposer owner={owner} images={false} onSaved={onSaved} onClose={onClose} />)
  await act(async () => {})
  fireEvent.change(screen.getByRole("textbox", { name: "Comment" }), { target: { value: "New unsent draft" } })

  await act(async () => complete({ ok: true, data: { id: 10, content: original.content } as any }))

  expect(onSaved).toHaveBeenCalledWith({ id: 10, content: original.content })
  expect(onClose).not.toHaveBeenCalled()
  expect(screen.getByRole("textbox", { name: "Comment" })).toHaveValue("New unsent draft")
})

test("a restored pending submission recovers the other tab's receipt without another request", async () => {
  jest.mocked(commentDrafts.load).mockResolvedValue([{ ...original, state: "pending" }])
  jest.mocked(commentDrafts.save).mockResolvedValue({
    scope: original.scope,
    submissionId: original.submissionId,
    state: "completed",
    updatedAt: 2,
    comment: { id: 9, content: original.content } as any,
  })

  const onSaved = jest.fn()
  render(<CommentComposer owner={owner} images={false} onSaved={onSaved} />)
  await act(async () => {})

  expect(screen.getByRole("textbox", { name: "Comment" })).toBeDisabled()
  fireEvent.click(screen.getByRole("button", { name: "Retry submission" }))
  await act(async () => {})

  expect(submitComment).not.toHaveBeenCalled()
  expect(onSaved).toHaveBeenCalledWith({ id: 9, content: original.content })
  expect(screen.getByRole("textbox", { name: "Comment" })).toHaveValue("")
})

test("receipt recovery leaves one attachment chooser and a usable composer for the next comment", async () => {
  jest.mocked(commentDrafts.load).mockResolvedValue([{ ...original, state: "pending" }])
  jest.mocked(commentDrafts.save).mockResolvedValueOnce({
    scope: original.scope,
    submissionId: original.submissionId,
    state: "completed",
    updatedAt: 2,
    comment: { id: 9, content: original.content } as any,
  })

  const onSaved = jest.fn()
  render(<CommentComposer owner={owner} images onSaved={onSaved} />)
  await act(async () => {})

  fireEvent.click(screen.getByRole("button", { name: "Retry submission" }))
  await act(async () => {})

  expect(screen.getAllByRole("textbox", { name: "Comment" })).toHaveLength(1)
  expect(screen.getAllByTestId("attachments")).toHaveLength(1)

  fireEvent.change(screen.getByRole("textbox", { name: "Comment" }), { target: { value: "Next comment" } })
  fireEvent.click(screen.getByRole("button", { name: "Submit comment" }))
  await act(async () => {})

  expect(submitComment).toHaveBeenCalledTimes(1)
  expect(onSaved).toHaveBeenLastCalledWith({ id: 10, content: "Next comment" })
  expect(screen.getAllByRole("textbox", { name: "Comment" })).toHaveLength(1)
  expect(screen.getByRole("textbox", { name: "Comment" })).toHaveValue("")
})
