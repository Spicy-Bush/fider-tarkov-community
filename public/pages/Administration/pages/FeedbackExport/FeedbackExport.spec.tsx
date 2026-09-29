import React from "react"
import { webcrypto } from "node:crypto"
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react"
import { FeedbackExportPreset, FeedbackExportRecipe } from "@fider/models"
import { Fider, http } from "@fider/services"
import * as actions from "@fider/services/actions/feedbackExport"
import { RequestError, Result } from "@fider/services/http"
import FeedbackExportPage from "./FeedbackExport.page"
import { newSection } from "./recipes"

jest.mock("@fider/services/notify", () => ({ success: jest.fn(), error: jest.fn() }))

Object.defineProperty(globalThis, "crypto", { value: webcrypto })

const recipe = (): FeedbackExportRecipe => ({ sections: [newSection("Top voted")] })
const preset = (id: string, name: string): FeedbackExportPreset => ({
  id,
  name,
  recipe: recipe(),
  updatedAt: "2026-09-28T00:00:00Z",
  updatedBy: "Admin",
})

const first = preset("a".repeat(32), "First")
const second = preset("b".repeat(32), "Second")

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => {
    resolve = done
  })
  return { promise, resolve }
}

function open(presets: FeedbackExportPreset[] = []) {
  return render(<FeedbackExportPage tags={[]} presets={presets} statuses={["open", "planned", "started"]} limits={{ rows: 500, pickSize: 200, sections: 10, picks: 5 }} />)
}

beforeEach(() => {
  localStorage.clear()
  Fider.initialize({ settings: { baseURL: "http://localhost", environment: "development" }, tenant: {}, user: { id: 7 } })
  jest.spyOn(window, "confirm").mockReturnValue(true)
  jest.spyOn(actions, "previewFeedbackExport").mockResolvedValue({ ok: true, data: { sections: [[]] } })
})

afterEach(() => {
  cleanup()
  jest.restoreAllMocks()
})

test("an untouched starter saves once and preserves later typing until the next save", async () => {
  const pending = deferred<Result<FeedbackExportPreset>>()
  const create = jest.spyOn(actions, "createFeedbackExportPreset").mockReturnValue(pending.promise)
  const update = jest.spyOn(actions, "updateFeedbackExportPreset").mockImplementation(async (_, sent) => ({
    ok: true,
    data: { preset: { ...sent, updatedAt: first.updatedAt, updatedBy: "Admin" }, conflicts: [] },
  }))
  open()

  expect(screen.getByRole("button", { name: "Save", exact: true })).toBeEnabled()
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  const sent = create.mock.calls[0][0]
  expect(sent.id).toMatch(/^[0-9a-f]{32}$/)

  fireEvent.change(screen.getByRole("textbox", { name: "Preset name" }), { target: { value: "Later name" } })
  fireEvent.change(screen.getAllByRole("textbox", { name: "Section name" })[0], { target: { value: "Later section" } })
  await act(async () => pending.resolve({ ok: true, data: { ...sent, updatedAt: first.updatedAt, updatedBy: "Admin" } }))

  expect(screen.getByRole("textbox", { name: "Preset name" })).toHaveValue("Later name")
  expect(screen.getAllByRole("textbox", { name: "Section name" })[0]).toHaveValue("Later section")
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  await waitFor(() => expect(update).toHaveBeenCalledTimes(1))
  expect(update.mock.calls[0][1]).toMatchObject({ id: sent.id, name: "Later name" })
  expect(update.mock.calls[0][1].recipe.sections[0].name).toBe("Later section")
  expect(update.mock.calls[0][2]).toEqual(sent)
  await waitFor(() => expect(screen.getByRole("button", { name: "Save", exact: true })).toBeDisabled())
})

test("a late save receipt does not reopen a preset after switching", async () => {
  const pending = deferred<Result<actions.PresetUpdateResult>>()
  jest.spyOn(actions, "updateFeedbackExportPreset").mockReturnValue(pending.promise)
  open([first, second])
  fireEvent.change(screen.getByRole("textbox", { name: "Preset name" }), { target: { value: "First edited" } })
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  fireEvent.change(screen.getByRole("combobox", { name: "Preset" }), { target: { value: second.id } })
  await act(async () => pending.resolve({ ok: true, data: { preset: { ...first, name: "First edited" }, conflicts: [] } }))

  expect(screen.getByRole("textbox", { name: "Preset name" })).toHaveValue("Second")
  expect(screen.getByRole("combobox", { name: "Preset" })).toHaveValue(second.id)
  expect(localStorage.getItem("fider:bsg-export:preset")).toBe(second.id)
})

test("uncertain creation retries its original identity and payload while preserving later edits", async () => {
  const transport = new RequestError("POST", "/api/admin/bsg-export/presets", "transport", new Error("offline"))
  const post = jest.spyOn(http, "post").mockRejectedValue(transport)
  open()
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  await screen.findByRole("button", { name: "Retry save" })
  expect(post).toHaveBeenCalledTimes(2)
  const original = post.mock.calls[0][1]
  expect(post.mock.calls[1][1]).toEqual(original)

  fireEvent.change(screen.getByRole("textbox", { name: "Preset name" }), { target: { value: "Newer input" } })
  post.mockResolvedValue({ ok: true, data: { ...original, updatedAt: first.updatedAt, updatedBy: "Admin" } })
  fireEvent.click(screen.getByRole("button", { name: "Retry save" }))
  await waitFor(() => expect(screen.queryByRole("button", { name: "Retry save" })).not.toBeInTheDocument())

  expect(post.mock.calls[2][1]).toEqual(original)
  expect(screen.getByRole("textbox", { name: "Preset name" })).toHaveValue("Newer input")
  await waitFor(() => expect(screen.getByRole("button", { name: "Save", exact: true })).toBeEnabled())
})

test("preview retries without changing the recipe or random seed", async () => {
  const preview = jest.mocked(actions.previewFeedbackExport)
  preview.mockResolvedValueOnce({ ok: false, status: 503, error: { errors: [{ message: "Unavailable" }] } })
  preview.mockResolvedValue({ ok: true, data: { sections: [[{ number: 1, title: "Post", slug: "post", votes: 3, comments: 2, tagIds: [], pick: "top" }]] } })
  open([first])
  fireEvent.click(await screen.findByRole("button", { name: "Retry preview" }))
  await waitFor(() => expect(screen.getByRole("button", { name: "CSV" })).toBeEnabled())
  expect(preview.mock.calls[1].slice(0, 2)).toEqual(preview.mock.calls[0].slice(0, 2))
  expect(screen.queryByRole("alert")).not.toBeInTheDocument()
})

test("a superseded preview failure cannot replace the current preview", async () => {
  const pending = deferred<Awaited<ReturnType<typeof actions.previewFeedbackExport>>>()
  const preview = jest.mocked(actions.previewFeedbackExport)
  preview.mockReturnValueOnce(pending.promise)
  open([first])
  await waitFor(() => expect(preview).toHaveBeenCalledTimes(1))
  fireEvent.change(screen.getByRole("spinbutton", { name: "Posts", exact: true }), { target: { value: "5" } })
  await waitFor(() => expect(preview).toHaveBeenCalledTimes(2))
  await act(async () => pending.resolve({ ok: false, status: 503, error: { errors: [{ message: "Old failure" }] } }))

  expect(preview.mock.calls[0][2].aborted).toBe(true)
  expect(screen.queryByText("Old failure")).not.toBeInTheDocument()
})


test("access loss after an uncertain create retains the original retry intent", async () => {
  const post = jest.spyOn(http, "post")
    .mockRejectedValueOnce(new RequestError("POST", "/api/admin/bsg-export/presets", "transport", new Error("lost response")))
    .mockResolvedValueOnce({ ok: false, status: 403, error: { errors: [{ message: "Access denied" }] } })
  open()
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  await screen.findByRole("alert")
  const accepted = post.mock.calls[0][1]
  fireEvent.change(screen.getByRole("textbox", { name: "Preset name" }), { target: { value: "Later draft" } })

  post.mockResolvedValue({ ok: true, data: { ...accepted, updatedAt: first.updatedAt, updatedBy: "Admin" } })
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  await waitFor(() => expect(post).toHaveBeenCalledTimes(3))
  await waitFor(() => expect(screen.getByRole("button", { name: "Save", exact: true })).toBeEnabled())

  expect(post.mock.calls[2][1]).toEqual(accepted)
  expect(screen.getByRole("textbox", { name: "Preset name" })).toHaveValue("Later draft")
})

test("an uncertain update retries the original baseline and preserves later typing", async () => {
  const update = jest.spyOn(actions, "updateFeedbackExportPreset")
    .mockRejectedValueOnce(new RequestError("PUT", "/api/admin/bsg-export/presets/id", "transport", new Error("lost response")))
  open([first])
  fireEvent.change(screen.getByRole("textbox", { name: "Preset name" }), { target: { value: "Sent name" } })
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  await screen.findByRole("button", { name: "Retry save" })

  fireEvent.change(screen.getByRole("textbox", { name: "Preset name" }), { target: { value: "Later name" } })
  update.mockResolvedValue({ ok: true, data: { preset: { ...first, name: "Sent name" }, conflicts: [] } })
  fireEvent.click(screen.getByRole("button", { name: "Retry save" }))
  await waitFor(() => expect(update).toHaveBeenCalledTimes(2))

  expect(update.mock.calls[1]).toEqual(update.mock.calls[0])
  expect(update.mock.calls[0][2]).toMatchObject({ name: "First", recipe: first.recipe })
  expect(screen.getByRole("textbox", { name: "Preset name" })).toHaveValue("Later name")
})

test.each(["Save my changes", "Use saved changes"])("conflicting updates preserve drafts and resolve through %s", async (choice) => {
  const current = { ...first, name: "Other admin" }
  const update = jest.spyOn(actions, "updateFeedbackExportPreset")
    .mockResolvedValueOnce({ ok: true, data: { preset: current, conflicts: ["name"] } })
    .mockImplementation(async (_, sent) => ({ ok: true, data: { preset: { ...first, ...sent }, conflicts: [] } }))
  open([first])
  fireEvent.change(screen.getByRole("textbox", { name: "Preset name" }), { target: { value: "My name" } })
  fireEvent.change(screen.getByRole("textbox", { name: "Section name" }), { target: { value: "My section" } })
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  await screen.findByRole("button", { name: "Save my changes" })

  expect(screen.getByRole("textbox", { name: "Preset name" })).toHaveValue("My name")
  expect(screen.getByRole("textbox", { name: "Section name" })).toHaveValue("My section")
  expect(screen.getByRole("button", { name: "Save", exact: true })).toBeDisabled()
  expect(screen.getByText("Other admin", { selector: "p" })).toBeInTheDocument()
  fireEvent.click(screen.getByRole("button", { name: choice }))

  if (choice === "Use saved changes") {
    expect(screen.getByRole("textbox", { name: "Preset name" })).toHaveValue("Other admin")
    expect(screen.getByRole("textbox", { name: "Section name" })).toHaveValue("My section")
    fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  }

  await waitFor(() => expect(update).toHaveBeenCalledTimes(2))
  expect(update.mock.calls[1][2]).toMatchObject({ name: "Other admin", recipe: first.recipe })
  expect(update.mock.calls[1][1].recipe.sections[0].name).toBe("My section")
  expect(update.mock.calls[1][1].name).toBe(choice === "Save my changes" ? "My name" : "Other admin")
  await waitFor(() => expect(screen.queryByRole("button", { name: "Save my changes" })).not.toBeInTheDocument())
})

test.each([
  { field: "name", submitted: false },
  { field: "name", submitted: true },
  { field: "recipe", submitted: false },
  { field: "recipe", submitted: true },
])("pending $field edits keep their actual baseline (submitted: $submitted)", async ({ field, submitted }) => {
  const pending = deferred<Result<actions.PresetUpdateResult>>()
  const update = jest.spyOn(actions, "updateFeedbackExportPreset")
    .mockReturnValueOnce(pending.promise)
    .mockResolvedValue({ ok: false, status: 503, error: { errors: [{ message: "Unavailable" }] } })
  open([first])

  const name = screen.getByRole("textbox", { name: "Preset name" })
  const section = screen.getByRole("textbox", { name: "Section name" })
  const edited = field === "name" ? name : section
  const other = field === "name" ? section : name
  fireEvent.change(submitted ? edited : other, { target: { value: "Submitted" } })
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  const sent = update.mock.calls[0][1]
  fireEvent.change(edited, { target: { value: "Still typing" } })

  const accepted = { ...first, ...sent, recipe: { sections: sent.recipe.sections.map((item) => ({ ...item })) } }
  if (!submitted) {
    if (field === "name") {
      accepted.name = "Other admin"
    } else {
      accepted.recipe.sections[0].name = "Other admin"
    }
  }
  await act(async () => pending.resolve({ ok: true, data: { preset: accepted, conflicts: [] } }))
  expect(edited).toHaveValue("Still typing")

  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  await waitFor(() => expect(update).toHaveBeenCalledTimes(2))
  expect(update.mock.calls[1][2][field]).toEqual(sent[field])
  expect(update.mock.calls[1][1][field]).not.toEqual(sent[field])
})

test("a recovered create receipt does not change the baseline for edits made while retrying", async () => {
  const pending = deferred<Result<FeedbackExportPreset>>()
  const create = jest.spyOn(actions, "createFeedbackExportPreset").mockReturnValue(pending.promise)
  const update = jest.spyOn(actions, "updateFeedbackExportPreset")
    .mockResolvedValue({ ok: false, status: 503, error: { errors: [{ message: "Unavailable" }] } })
  open()
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  const sent = create.mock.calls[0][0]
  fireEvent.change(screen.getByRole("textbox", { name: "Preset name" }), { target: { value: "Pending draft" } })
  await act(async () => pending.resolve({ ok: true, data: { ...first, ...sent, name: "Changed after creation" } }))
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))

  await waitFor(() => expect(update).toHaveBeenCalledTimes(1))
  expect(update.mock.calls[0][2].name).toBe(sent.name)
  expect(update.mock.calls[0][1].name).toBe("Pending draft")
})

test("section labels update without selecting the same posts again", async () => {
  const preview = jest.mocked(actions.previewFeedbackExport)
  open([first])
  await waitFor(() => expect(preview).toHaveBeenCalledTimes(1))
  fireEvent.change(screen.getByRole("textbox", { name: "Section name" }), { target: { value: "New heading" } })
  await act(async () => new Promise((resolve) => window.setTimeout(resolve, 350)))

  expect(preview).toHaveBeenCalledTimes(1)
  expect(screen.getByRole("textbox", { name: "Section name" })).toHaveValue("New heading")
})

test.each(["Save my changes", "Use saved changes"])("resolving a name conflict does not rebase unseen recipe edits through %s", async (choice) => {
  const pending = deferred<Result<actions.PresetUpdateResult>>()
  const update = jest.spyOn(actions, "updateFeedbackExportPreset")
    .mockReturnValueOnce(pending.promise)
    .mockResolvedValue({ ok: false, status: 503, error: { errors: [{ message: "Unavailable" }] } })
  open([first])
  fireEvent.change(screen.getByRole("textbox", { name: "Preset name" }), { target: { value: "My name" } })
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  fireEvent.change(screen.getByRole("textbox", { name: "Section name" }), { target: { value: "Still editing" } })
  const current = { ...first, name: "Other name", recipe: { sections: [newSection("Other section")] } }
  await act(async () => pending.resolve({ ok: true, data: { preset: current, conflicts: ["name"] } }))
  fireEvent.click(screen.getByRole("button", { name: choice }))
  if (choice === "Use saved changes") {
    fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  }

  await waitFor(() => expect(update).toHaveBeenCalledTimes(2))
  expect(update.mock.calls[1][1].recipe.sections[0].name).toBe("Still editing")
  expect(update.mock.calls[1][2].recipe).toEqual(first.recipe)
  expect(update.mock.calls[1][2].name).toBe("Other name")
})

test("resolving a conflict adopts unrelated server fields without marking them edited", async () => {
  const current = { ...first, name: "Other name", recipe: { sections: [newSection("Other section")] } }
  jest.spyOn(actions, "updateFeedbackExportPreset")
    .mockResolvedValue({ ok: true, data: { preset: current, conflicts: ["name"] } })
  open([first])
  fireEvent.change(screen.getByRole("textbox", { name: "Preset name" }), { target: { value: "My name" } })
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  fireEvent.click(await screen.findByRole("button", { name: "Use saved changes" }))

  expect(screen.getByRole("textbox", { name: "Section name" })).toHaveValue("Other section")
  expect(screen.getByRole("button", { name: "Save", exact: true })).toBeDisabled()
})

test.each(["name", "recipe"])("a pending %s revert survives a conflict in another field", async (field) => {
  const pending = deferred<Result<actions.PresetUpdateResult>>()
  const update = jest.spyOn(actions, "updateFeedbackExportPreset")
    .mockReturnValueOnce(pending.promise)
    .mockResolvedValue({ ok: false, status: 503, error: { errors: [{ message: "Unavailable" }] } })
  open([first])
  const name = screen.getByRole("textbox", { name: "Preset name" })
  const section = screen.getByRole("textbox", { name: "Section name" })
  fireEvent.change(name, { target: { value: "Submitted name" } })
  fireEvent.change(section, { target: { value: "Submitted section" } })
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  const sent = update.mock.calls[0][1]
  const edited = field === "name" ? name : section
  const original = field === "name" ? first.name : first.recipe.sections[0].name
  fireEvent.change(edited, { target: { value: original } })

  const accepted = { ...first, ...sent }
  if (field === "name") {
    accepted.recipe = { sections: [newSection("Other section")] }
  } else {
    accepted.name = "Other name"
  }
  const conflicts: actions.PresetUpdateResult["conflicts"] = field === "name" ? ["recipe"] : ["name"]
  await act(async () => pending.resolve({ ok: true, data: { preset: accepted, conflicts } }))
  expect(edited).toHaveValue(original)
  fireEvent.click(screen.getByRole("button", { name: "Use saved changes" }))
  expect(edited).toHaveValue(original)
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))

  await waitFor(() => expect(update).toHaveBeenCalledTimes(2))
  expect(update.mock.calls[1][1][field]).toEqual(first[field])
  expect(update.mock.calls[1][2][field]).toEqual(sent[field])
})

test("server normalization confirms pending baselines during an unrelated conflict", async () => {
  const pending = deferred<Result<actions.PresetUpdateResult>>()
  const update = jest.spyOn(actions, "updateFeedbackExportPreset")
    .mockReturnValueOnce(pending.promise)
    .mockResolvedValue({ ok: false, status: 503, error: { errors: [{ message: "Unavailable" }] } })
  open([first])
  fireEvent.change(screen.getByRole("textbox", { name: "Preset name" }), { target: { value: "  Submitted  " } })
  fireEvent.change(screen.getByRole("textbox", { name: "Section name" }), { target: { value: "Submitted section" } })
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  const sent = update.mock.calls[0][1]
  fireEvent.change(screen.getByRole("textbox", { name: "Preset name" }), { target: { value: "Pending name" } })
  const accepted = {
    ...first,
    name: "Submitted",
    recipe: { sections: [{ ...sent.recipe.sections[0], name: "Other section", statuses: ["open", "started", "planned"] as const }] },
  }
  await act(async () => pending.resolve({ ok: true, data: { preset: accepted as FeedbackExportPreset, conflicts: ["recipe"] } }))
  fireEvent.click(screen.getByRole("button", { name: "Use saved changes" }))
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  await waitFor(() => expect(update).toHaveBeenCalledTimes(2))

  expect(update.mock.calls[1][2].name).toBe("Submitted")
  expect(update.mock.calls[1][1].name).toBe("Pending name")
})

test("server filter ordering confirms a submitted recipe before further local edits", async () => {
  const pending = deferred<Result<actions.PresetUpdateResult>>()
  const update = jest.spyOn(actions, "updateFeedbackExportPreset")
    .mockReturnValueOnce(pending.promise)
    .mockResolvedValue({ ok: false, status: 503, error: { errors: [{ message: "Unavailable" }] } })
  open([first])
  fireEvent.change(screen.getByRole("textbox", { name: "Preset name" }), { target: { value: "Submitted" } })
  fireEvent.change(screen.getByRole("textbox", { name: "Section name" }), { target: { value: "Submitted section" } })
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  const sent = update.mock.calls[0][1]
  fireEvent.change(screen.getByRole("textbox", { name: "Section name" }), { target: { value: "Pending section" } })
  const accepted: FeedbackExportPreset = {
    ...first,
    name: "Other name",
    recipe: { sections: [{ ...sent.recipe.sections[0], statuses: ["open", "started", "planned"] }] },
  }
  await act(async () => pending.resolve({ ok: true, data: { preset: accepted, conflicts: ["name"] } }))
  fireEvent.click(screen.getByRole("button", { name: "Use saved changes" }))
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  await waitFor(() => expect(update).toHaveBeenCalledTimes(2))

  expect(update.mock.calls[1][2].recipe.sections[0].name).toBe("Submitted section")
  expect(update.mock.calls[1][1].recipe.sections[0].name).toBe("Pending section")
})

test("export controls use the server limits", () => {
  render(<FeedbackExportPage
    tags={[]}
    presets={[first]}
    statuses={["open"]}
    limits={{ rows: 2, pickSize: 3, sections: 1, picks: 1 }}
  />)

  expect(screen.queryByRole("button", { name: "Add section" })).not.toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Add pick" })).not.toBeInTheDocument()
  expect(screen.getByText("An export can have at most 2 posts.")).toBeInTheDocument()

  const count = screen.getByRole("spinbutton", { name: "Posts" })
  expect(count).toHaveAttribute("max", "3")
  fireEvent.change(count, { target: { value: "8" } })
  expect(count).toHaveValue(3)
})

test.each(["name", "recipe"] as const)("a receipt retry retains later %s edits with their sent baseline", async (field) => {
  const failure = new RequestError("PUT", "/api/admin/bsg-export/presets", "transport", new Error("lost acknowledgement"))
  const retry = deferred<Result<actions.PresetUpdateResult>>()
  const update = jest.spyOn(actions, "updateFeedbackExportPreset")
    .mockRejectedValueOnce(failure)
    .mockReturnValueOnce(retry.promise)
    .mockResolvedValue({ ok: false, status: 503, error: { errors: [{ message: "Unavailable" }] } })
  open([first])

  const input = screen.getByRole("textbox", { name: field === "name" ? "Preset name" : "Section name" })
  fireEvent.change(input, { target: { value: "Submitted" } })
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  await screen.findByRole("button", { name: "Retry save" })

  const sent = update.mock.calls[0][1]
  fireEvent.change(input, { target: { value: "Later edit" } })
  fireEvent.click(screen.getByRole("button", { name: "Retry save" }))

  const accepted = { ...first, ...sent }
  await act(async () => retry.resolve({ ok: true, data: { preset: accepted, conflicts: [] } }))
  expect(input).toHaveValue("Later edit")
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))

  await waitFor(() => expect(update).toHaveBeenCalledTimes(3))
  expect(update.mock.calls[1][0]).toEqual(update.mock.calls[0][0])
  expect(update.mock.calls[2][0]).not.toEqual(update.mock.calls[0][0])
  expect(update.mock.calls[2][2][field]).toEqual(sent[field])
})

test.each(["name", "recipe"] as const)("a recovered create cannot rebase later %s edits over another admin", async (field) => {
  const failure = new RequestError("POST", "/api/admin/bsg-export/presets", "transport", new Error("lost acknowledgement"))
  const retry = deferred<Result<FeedbackExportPreset>>()
  const create = jest.spyOn(actions, "createFeedbackExportPreset")
    .mockRejectedValueOnce(failure)
    .mockReturnValueOnce(retry.promise)
  const update = jest.spyOn(actions, "updateFeedbackExportPreset")
    .mockResolvedValue({ ok: false, status: 503, error: { errors: [{ message: "Unavailable" }] } })
  open()

  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  await screen.findByRole("button", { name: "Retry save" })
  const sent = create.mock.calls[0][0]
  const input = screen.getAllByRole("textbox", { name: field === "name" ? "Preset name" : "Section name" })[0]
  fireEvent.change(input, { target: { value: "Later edit" } })
  fireEvent.click(screen.getByRole("button", { name: "Retry save" }))

  const accepted = { ...first, ...sent }
  await act(async () => retry.resolve({ ok: true, data: accepted }))
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))

  await waitFor(() => expect(update).toHaveBeenCalledTimes(1))
  expect(update.mock.calls[0][2][field]).toEqual(sent[field])
})

test("a rejection after a lost update response preserves later typing and the original receipt", async () => {
  const update = jest.spyOn(actions, "updateFeedbackExportPreset")
    .mockRejectedValueOnce(new RequestError("PUT", "/api/admin/bsg-export/presets", "transport", new Error("lost response")))
    .mockResolvedValueOnce({ ok: false, status: 403, error: { errors: [{ message: "Unavailable" }] } })
    .mockResolvedValue({ ok: true, data: { preset: { ...first, name: "Accepted" }, conflicts: [] } })
  open([first])
  fireEvent.change(screen.getByRole("textbox", { name: "Preset name" }), { target: { value: "Accepted" } })
  fireEvent.click(screen.getByRole("button", { name: "Save", exact: true }))
  await screen.findByRole("button", { name: "Retry save" })

  fireEvent.change(screen.getByRole("textbox", { name: "Preset name" }), { target: { value: "Still editing" } })
  fireEvent.click(screen.getByRole("button", { name: "Retry save" }))
  await screen.findByText("Unavailable")
  fireEvent.click(screen.getByRole("button", { name: "Retry save" }))
  await waitFor(() => expect(update).toHaveBeenCalledTimes(3))

  expect(update.mock.calls[1]).toEqual(update.mock.calls[0])
  expect(update.mock.calls[2]).toEqual(update.mock.calls[0])
  expect(screen.getByRole("textbox", { name: "Preset name" })).toHaveValue("Still editing")
})
