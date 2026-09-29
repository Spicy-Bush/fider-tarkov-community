import React from "react"
import { webcrypto } from "node:crypto"
import { afterEach, beforeEach, expect, test } from "@jest/globals"
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react"
import { PermissionAssignment, PermissionRequirements, PostStatusValue, RolePermissionLocks, RolePermissions, SavedRolePermissions } from "@fider/models"
import { Fider, http } from "@fider/services"
import { RequestError } from "@fider/services/http"
import ManagePermissionsPage from "./ManagePermissions.page"
import { ALL_PERMISSIONS } from "./catalog"

jest.mock("@fider/services/notify", () => ({ success: jest.fn(), error: jest.fn() }))

Object.defineProperty(globalThis, "crypto", { value: webcrypto })

// jsdom has no PointerEvent; without it fireEvent drops button and modifier keys.
class TestPointerEvent extends MouseEvent {
  readonly pointerType: string
  constructor(type: string, init: PointerEventInit = {}) {
    super(type, init)
    this.pointerType = init.pointerType ?? "mouse"
  }
}
Object.assign(window, { PointerEvent: TestPointerEvent })

const DRAFT_KEY = "fider:permissions-draft:7"

const serverPermissions = (): RolePermissions => ({
  visitor: ["createPosts", "editPosts"],
  helper: ["createPosts", "editPosts", "manageQueue", "tagPosts"],
  moderator: ["createPosts", "editPosts", "manageQueue", "tagPosts", "manageReports", "readProfiles"],
  collaborator: ["createPosts", "editPosts", "readSettings"],
  administrator: [...ALL_PERMISSIONS],
})

const EVERY = "Administrators have every permission"
const ADMIN_LOCKS: RolePermissionLocks = { administrator: Object.fromEntries(ALL_PERMISSIONS.map((permission) => [permission, EVERY])) }

const responseState = (): Pick<SavedRolePermissions, "responseOptions" | "responses" | "defaultResponses" | "responseLocks"> => ({
  responseOptions: [PostStatusValue.Open, PostStatusValue.Planned, PostStatusValue.Started, PostStatusValue.Completed, PostStatusValue.Declined, PostStatusValue.Duplicate],
  responses: { visitor: [], helper: [], moderator: [], collaborator: [], administrator: Object.values(PostStatusValue).filter((status) => status !== PostStatusValue.Deleted && status !== PostStatusValue.Archived) },
  defaultResponses: { visitor: [], helper: [], moderator: [], collaborator: [], administrator: [] },
  responseLocks: {},
})

const pageProps = (permissions: RolePermissions) => ({
  ...responseState(),
  permissions,
  defaults: serverPermissions(),
  requires: { manageMembers: ["readProfiles"], changeUserRoles: ["manageMembers"] } as PermissionRequirements,
  baseLocks: ADMIN_LOCKS,
})

const cell = (role: string, permission: string): HTMLElement => {
  const element = document.querySelector<HTMLElement>(`td[role="gridcell"][aria-label^="${role}: ${permission},"]`)
  if (!element) throw new Error(`No cell for ${role}: ${permission}`)
  return element
}

const isGranted = (role: string, permission: string) => /, granted/.test(cell(role, permission).getAttribute("aria-label") || "")

const click = async (element: Element, init: Partial<PointerEventInit> = {}) => {
  await act(async () => {
    fireEvent.pointerDown(element, { button: 0, pointerType: "mouse", ...init })
    fireEvent.pointerUp(window)
  })
}

let requests: unknown[] = []
let respond: ((value: unknown) => void) | undefined

beforeEach(() => {
  Element.prototype.scrollIntoView = jest.fn()
  localStorage.clear()
  requests = []
  Fider.initialize({ settings: { environment: "development" }, tenant: {}, user: { id: 7, name: "Admin" } })
  http.post = jest.fn()
  http.put = jest.fn((url: string, body: unknown) => {
    requests.push({ url, body })
    return new Promise((resolve) => (respond = resolve))
  }) as typeof http.put
})

afterEach(() => {
  cleanup()
  jest.restoreAllMocks()
})

test("save sends only changed cells and keeps edits made while it was in flight", async () => {
  await act(async () => {
    render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  })

  await click(cell("Visitor", "Lock posts"))
  expect(isGranted("Visitor", "Lock posts")).toBe(true)
  expect(screen.getByText("1 unsaved change")).toBeInTheDocument()

  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Save" })))
  expect(requests).toEqual([{ url: "/api/admin/permissions", body: { submissionId: expect.any(String), responseChanges: [], changes: [{ role: "visitor", permission: "lockPosts", granted: true }] } }])

  await click(cell("Helper", "Lock posts"))
  const server = serverPermissions()
  server.visitor.push("lockPosts", "tagPosts")
  await act(async () => respond?.({ ok: true, data: { ...responseState(), permissions: server, baseLocks: ADMIN_LOCKS, requires: pageProps(server).requires } }))

  expect(isGranted("Visitor", "Tag posts")).toBe(true)
  expect(isGranted("Helper", "Lock posts")).toBe(true)
  expect(screen.getByText("1 unsaved change")).toBeInTheDocument()
  expect(JSON.parse(localStorage.getItem(DRAFT_KEY) || "{}").changes).toEqual([{ role: "helper", permission: "lockPosts", granted: true }])
})

test("undo and redo preserve unrelated grants accepted with a save", async () => {
  await act(async () => {
    render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  })
  await click(cell("Visitor", "Lock posts"))
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Save" })))

  const server = serverPermissions()
  server.visitor.push("lockPosts", "tagPosts")
  await act(async () => respond?.({ ok: true, data: { ...responseState(), permissions: server, baseLocks: ADMIN_LOCKS, requires: pageProps(server).requires } }))

  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Undo" })))
  expect(isGranted("Visitor", "Lock posts")).toBe(false)
  expect(isGranted("Visitor", "Tag posts")).toBe(true)
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Save" })))
  expect(requests[1]).toEqual({
    url: "/api/admin/permissions",
    body: { submissionId: expect.any(String), responseChanges: [], changes: [{ role: "visitor", permission: "lockPosts", granted: false }] },
  })

  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Redo" })))
  expect(isGranted("Visitor", "Lock posts")).toBe(true)
  expect(isGranted("Visitor", "Tag posts")).toBe(true)
})

test("undo during a save remains reversible after the server receipt", async () => {
  await act(async () => {
    render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  })
  await click(cell("Visitor", "Lock posts"))
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Save" })))
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Undo" })))

  const server = serverPermissions()
  server.visitor.push("lockPosts", "tagPosts")
  await act(async () => respond?.({ ok: true, data: { ...responseState(), permissions: server, baseLocks: ADMIN_LOCKS, requires: pageProps(server).requires } }))

  expect(isGranted("Visitor", "Lock posts")).toBe(false)
  expect(isGranted("Visitor", "Tag posts")).toBe(true)
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Redo" })))
  expect(isGranted("Visitor", "Lock posts")).toBe(true)
  expect(isGranted("Visitor", "Tag posts")).toBe(true)
})

test("Ctrl and Shift clicks select cells without toggling their grants", async () => {
  await act(async () => {
    render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  })
  await click(cell("Visitor", "Lock posts"), { ctrlKey: true })
  expect(isGranted("Visitor", "Lock posts")).toBe(false)

  await click(cell("Visitor", "Tag posts"), { shiftKey: true })
  expect(document.querySelectorAll('td[aria-selected="true"]')).toHaveLength(2)
  expect(isGranted("Visitor", "Lock posts")).toBe(false)
  expect(isGranted("Visitor", "Tag posts")).toBe(false)
  expect(screen.queryByRole("button", { name: "Save" })).toBeNull()
})

test("unsaved changes survive leaving the page and apply over newer server data", async () => {
  const first = render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  await click(cell("Moderator", "Tag posts"))
  expect(isGranted("Moderator", "Tag posts")).toBe(false)
  first.unmount()

  const server = serverPermissions()
  server.collaborator.push("manageTags")
  await act(async () => {
    render(<ManagePermissionsPage {...pageProps(server)} />)
  })

  expect(screen.getByRole("status")).toHaveTextContent("Restored 1 unsaved change")
  expect(isGranted("Moderator", "Tag posts")).toBe(false)
  expect(isGranted("Collaborator", "Tags")).toBe(true)

  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Discard" })))
  expect(isGranted("Moderator", "Tag posts")).toBe(true)
  expect(localStorage.getItem(DRAFT_KEY)).toBeNull()

  await act(async () => fireEvent.keyDown(document.body, { key: "z", ctrlKey: true }))
  expect(isGranted("Moderator", "Tag posts")).toBe(false)
})

test("keyboard selection grants a block, and undo and redo walk the history", async () => {
  render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  const grid = screen.getByRole("grid")

  const start = cell("Visitor", "Respond: Open")
  await click(start, { ctrlKey: true })
  fireEvent.keyDown(start, { key: "ArrowDown", shiftKey: true })
  fireEvent.keyDown(start, { key: "ArrowRight", shiftKey: true })
  fireEvent.keyDown(grid, { key: "g" })

  for (const role of ["Visitor", "Helper"]) {
    expect(isGranted(role, "Respond: Open")).toBe(true)
    expect(isGranted(role, "Respond: Planned")).toBe(true)
    expect(isGranted(role, "Use canned responses")).toBe(true)
  }
  expect(screen.getByText("4 selected")).toBeInTheDocument()

  fireEvent.keyDown(document.body, { key: "z", ctrlKey: true })
  expect(isGranted("Helper", "Respond: Planned")).toBe(false)
  fireEvent.keyDown(document.body, { key: "z", ctrlKey: true, shiftKey: true })
  expect(isGranted("Helper", "Respond: Planned")).toBe(true)
})

test("a copied column pastes into another role", async () => {
  render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)

  fireEvent.pointerDown(screen.getByRole("columnheader", { name: /^Moderator/ }), { button: 0, pointerType: "mouse" })
  fireEvent.pointerUp(window)
  const grid = screen.getByRole("grid")
  fireEvent.keyDown(grid, { key: "c", ctrlKey: true })

  await click(cell("Visitor", "Create posts"), { ctrlKey: true })
  fireEvent.keyDown(grid, { key: "v", ctrlKey: true })

  for (const permission of ["Post queue", "Tag posts", "Reports", "View profiles"]) {
    expect(isGranted("Visitor", permission)).toBe(true)
  }
  expect(isGranted("Visitor", "Edit posts")).toBe(true)
})

test("administrator ordinary permissions stay fixed while response statuses remain editable", async () => {
  render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)

  expect(screen.getByRole("button", { name: "Administrator actions" })).toBeInTheDocument()

  await click(cell("Administrator", "Billing"))
  expect(cell("Administrator", "Billing")).toHaveAttribute("aria-label", "Administrator: Billing, granted, locked")
  expect(cell("Administrator", "Billing")).toHaveAttribute("data-tooltip", EVERY)
  expect(screen.queryByText(/unsaved change/)).toBeNull()
})


test("delegated role editing leaves authentication locked by server policy", async () => {
  const props = pageProps(serverPermissions())
  props.baseLocks = {
    ...ADMIN_LOCKS,
    helper: { manageAuthentication: "Only administrators can manage authentication" },
  }
  render(<ManagePermissionsPage {...props} />)

  expect(cell("Helper", "Authentication")).toHaveAttribute("aria-label", "Helper: Authentication, not granted, locked")
  await click(cell("Helper", "Authentication"))
  expect(isGranted("Helper", "Authentication")).toBe(false)

  await click(cell("Helper", "Role permissions"))
  expect(isGranted("Helper", "Role permissions")).toBe(true)
  expect(screen.getByText("1 unsaved change")).toBeInTheDocument()
})


test("blocked browser storage keeps edits editable and retries after recovery", async () => {
  render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  const write = jest.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
    throw new DOMException("Storage full", "QuotaExceededError")
  })
  await click(cell("Visitor", "Lock posts"))

  expect(isGranted("Visitor", "Lock posts")).toBe(true)
  expect(screen.getByRole("alert")).toHaveTextContent("Browser draft storage is unavailable")
  const leaving = new Event("beforeunload", { cancelable: true })
  window.dispatchEvent(leaving)
  expect(leaving.defaultPrevented).toBe(true)

  write.mockRestore()
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Retry draft storage" })))
  expect(screen.queryByRole("alert")).not.toBeInTheDocument()
  expect(JSON.parse(localStorage.getItem(DRAFT_KEY) || "{}").changes).toEqual([
    { role: "visitor", permission: "lockPosts", granted: true },
  ])
})

test("failed local cleanup cannot turn an accepted server save into unsaved work on restore", async () => {
  const first = render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  await click(cell("Visitor", "Lock posts"))
  const remove = jest.spyOn(Storage.prototype, "removeItem").mockImplementation(() => {
    throw new DOMException("Storage blocked", "SecurityError")
  })
  fireEvent.click(screen.getByRole("button", { name: "Save" }))
  const server = serverPermissions()
  server.visitor.push("lockPosts")
  await act(async () => respond?.({ ok: true, data: { ...responseState(), permissions: server, baseLocks: ADMIN_LOCKS, requires: pageProps(server).requires } }))

  expect(screen.queryByRole("button", { name: "Save", exact: true })).not.toBeInTheDocument()
  const leaving = new Event("beforeunload", { cancelable: true })
  window.dispatchEvent(leaving)
  expect(leaving.defaultPrevented).toBe(false)
  first.unmount()
  remove.mockRestore()
  render(<ManagePermissionsPage {...pageProps(server)} />)

  expect(isGranted("Visitor", "Lock posts")).toBe(true)
  expect(screen.queryByRole("button", { name: "Save", exact: true })).not.toBeInTheDocument()
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Retry save" })))
  await act(async () => respond?.({ ok: true, data: { ...responseState(), permissions: server, baseLocks: ADMIN_LOCKS, requires: pageProps(server).requires } }))
  expect(localStorage.getItem(DRAFT_KEY)).toBeNull()
})

test("a rejected save refreshes the authoritative locks and dependencies", async () => {
  await act(async () => {
    render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  })
  await click(cell("Visitor", "Lock posts"))
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Save", exact: true })))

  const locked = { ...ADMIN_LOCKS, visitor: { lockPosts: "Only higher roles can change this permission" } }
  await act(async () => respond?.({
    ok: true,
    data: {
      ...responseState(),
      permissions: serverPermissions(),
      baseLocks: locked,
      requires: { lockPosts: ["readResponses"] },
      blocked: "Only higher roles can change this permission",
    },
  }))

  expect(cell("Visitor", "Lock posts")).toHaveAttribute("aria-label", "Visitor: Lock posts, not granted, locked")
  await click(cell("Visitor", "Lock posts"))
  expect(isGranted("Visitor", "Lock posts")).toBe(false)

  await click(cell("Helper", "Lock posts"))
  expect(isGranted("Helper", "Use canned responses")).toBe(true)
})

test("restoring a local edit preserves its intended value", async () => {
  const first = render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  await click(cell("Helper", "Reports"))
  first.unmount()

  await act(async () => {
    render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  })
  expect(isGranted("Helper", "Reports")).toBe(true)
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Save", exact: true })))

  expect(requests[0]).toEqual({
    url: "/api/admin/permissions",
    body: { submissionId: expect.any(String), responseChanges: [], changes: [{ role: "helper", permission: "manageReports", granted: true }] },
  })
})

test("offline choices apply dependencies immediately and remain reversible after reload", async () => {
  const first = render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)

  await click(cell("Visitor", "Respond: Planned"))
  await click(cell("Visitor", "Respond: Planned"))
  expect(isGranted("Visitor", "Respond: Planned")).toBe(false)
  expect(isGranted("Visitor", "Use canned responses")).toBe(true)
  expect(http.post).not.toHaveBeenCalled()

  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Undo" })))
  expect(isGranted("Visitor", "Respond: Planned")).toBe(true)

  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Redo" })))
  expect(isGranted("Visitor", "Respond: Planned")).toBe(false)
  first.unmount()

  render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  expect(isGranted("Visitor", "Use canned responses")).toBe(true)
  expect(isGranted("Visitor", "Respond: Planned")).toBe(false)
  expect(http.post).not.toHaveBeenCalled()

  await act(async () => fireEvent.keyDown(document.body, { key: "s", ctrlKey: true }))
  expect(requests).toEqual([{
    url: "/api/admin/permissions",
    body: { submissionId: expect.any(String), responseChanges: [], changes: [{ role: "visitor", permission: "readResponses", granted: true }] },
  }])
})

test("effective locks prevent edits whose requirements are locked", async () => {
  const props = pageProps(serverPermissions())
  props.baseLocks = { ...ADMIN_LOCKS, visitor: { readResponses: "This requirement is locked" } }
  render(<ManagePermissionsPage {...props} />)

  expect(cell("Visitor", "Respond: Planned")).toHaveAttribute("data-tooltip", "This requirement is locked")
  await click(cell("Visitor", "Respond: Planned"))
  expect(isGranted("Visitor", "Respond: Planned")).toBe(false)
  expect(isGranted("Visitor", "Use canned responses")).toBe(false)
  expect(http.post).not.toHaveBeenCalled()
  expect(requests).toEqual([])
})

test("column actions send only cells declared editable by the server", async () => {
  const props = pageProps(serverPermissions())
  props.baseLocks = { ...ADMIN_LOCKS, visitor: { manageAuthentication: "Only administrators", exportBackup: "Only administrators" } }
  render(<ManagePermissionsPage {...props} />)

  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Visitor actions" })))
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Copy editable from Administrator" })))

  expect(http.post).not.toHaveBeenCalled()
  expect(isGranted("Visitor", "Files")).toBe(true)
  expect(isGranted("Visitor", "Authentication")).toBe(false)
  expect(isGranted("Visitor", "Full backups")).toBe(false)
})

test("reloading an unsaved choice permits a first save", async () => {
  const first = render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  await click(cell("Visitor", "Lock posts"))
  first.unmount()

  await act(async () => {
    render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  })
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Save", exact: true })))

  expect(requests).toEqual([{
    url: "/api/admin/permissions",
    body: { submissionId: expect.any(String), responseChanges: [], changes: [{ role: "visitor", permission: "lockPosts", granted: true }] },
  }])
})

test("new server locks retain independent editable choices after a rejected save", async () => {
  render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  await click(cell("Visitor", "Lock posts"))
  await click(cell("Helper", "Lock posts"))
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Save", exact: true })))

  const locked = { ...ADMIN_LOCKS, visitor: { lockPosts: "This choice is now locked" } }
  await act(async () => respond?.({
    ok: true,
    data: {
      ...responseState(),
      permissions: serverPermissions(),
      requires: pageProps(serverPermissions()).requires,
      baseLocks: locked,
      blocked: "This choice is now locked",
    },
  }))

  expect(isGranted("Visitor", "Lock posts")).toBe(false)
  expect(isGranted("Helper", "Lock posts")).toBe(true)
  expect(screen.getByRole("alert")).toHaveTextContent("This choice is now locked")
})

test("a choice made during save survives independent server changes", async () => {
  render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  await click(cell("Helper", "Lock posts"))
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Save", exact: true })))
  await click(cell("Visitor", "Lock posts"))

  const server = serverPermissions()
  server.helper.push("lockPosts")
  await act(async () => respond?.({
    ok: true,
    data: {
      ...responseState(),
      permissions: server,
      baseLocks: ADMIN_LOCKS,
      requires: pageProps(server).requires,
    },
  }))
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Save", exact: true })))

  expect(requests[1]).toEqual({
    url: "/api/admin/permissions",
    body: {
      submissionId: expect.any(String), responseChanges: [],
      changes: [{ role: "visitor", permission: "lockPosts", granted: true }],
    },
  })
})

test.each([false, true])("receipt retry after reload preserves a pending revert (server grant: %s)", async (serverGrant) => {
  const put = jest.mocked(http.put)
  put.mockRejectedValueOnce(new RequestError("PUT", "/api/admin/permissions", "transport", new Error("lost response")))
  const first = render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  await click(cell("Visitor", "Lock posts"))
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Save", exact: true })))
  const original = put.mock.calls[0][1]

  await click(cell("Visitor", "Lock posts"))
  first.unmount()
  const server = serverPermissions()
  if (serverGrant) {
    server.visitor.push("lockPosts")
  }
  render(<ManagePermissionsPage {...pageProps(server)} />)
  expect(isGranted("Visitor", "Lock posts")).toBe(false)

  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Retry save" })))
  expect(put.mock.calls[1][1]).toEqual(original)
  await act(async () => respond?.({ ok: true, data: { ...responseState(), permissions: server, baseLocks: ADMIN_LOCKS, requires: pageProps(server).requires } }))
  expect(isGranted("Visitor", "Lock posts")).toBe(false)

  if (serverGrant) {
    await act(async () => fireEvent.click(screen.getByRole("button", { name: "Save", exact: true })))
    expect(put.mock.calls[2][1]).toMatchObject({ changes: [{ role: "visitor", permission: "lockPosts", granted: false }] })
    expect(put.mock.calls[2][1]).not.toMatchObject({ submissionId: (original as { submissionId: string }).submissionId })
  } else {
    expect(screen.queryByRole("button", { name: "Save", exact: true })).not.toBeInTheDocument()
  }
})

test("a rejection after a lost save response preserves the pending choices and receipt", async () => {
  const put = jest.mocked(http.put)
  put.mockRejectedValueOnce(new RequestError("PUT", "/api/admin/permissions", "transport", new Error("lost response")))
  render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  await click(cell("Visitor", "Lock posts"))
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Save", exact: true })))
  const original = put.mock.calls[0][1]

  put.mockResolvedValueOnce({ ok: false, status: 403, error: { errors: [{ message: "Unavailable" }] } })
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Retry save" })))
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Retry save" })))
  expect(put.mock.calls[2][1]).toEqual(original)
  expect(isGranted("Visitor", "Lock posts")).toBe(true)
})

test("a definitive first rejection leaves editable choices for a new submission", async () => {
  const put = jest.mocked(http.put)
  put.mockResolvedValueOnce({ ok: false, status: 400, error: { errors: [{ message: "Rejected" }] } })
  render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  await click(cell("Visitor", "Lock posts"))
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Save", exact: true })))
  const original = put.mock.calls[0][1] as { submissionId: string }

  expect(isGranted("Visitor", "Lock posts")).toBe(true)
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Save", exact: true })))
  expect(put.mock.calls[1][1]).not.toMatchObject({ submissionId: original.submissionId })
})

test("response statuses share ordinary dependency undo and use the typed server contract", async () => {
  render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  expect(screen.queryByText("Respond to posts")).toBeNull()
  await click(cell("Moderator", "Respond: Planned"))
  await click(cell("Moderator", "Respond: Completed"))
  expect(isGranted("Moderator", "Use canned responses")).toBe(true)
  expect(isGranted("Moderator", "Respond: Open")).toBe(false)

  fireEvent.keyDown(document.body, { key: "z", ctrlKey: true })
  expect(isGranted("Moderator", "Respond: Completed")).toBe(false)
  expect(isGranted("Moderator", "Respond: Planned")).toBe(true)
  fireEvent.keyDown(document.body, { key: "z", ctrlKey: true, shiftKey: true })
  expect(isGranted("Moderator", "Respond: Completed")).toBe(true)

  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Save", exact: true })))
  expect(requests).toEqual([{
    url: "/api/admin/permissions",
    body: {
      submissionId: expect.any(String),
      changes: [{ role: "moderator", permission: "readResponses", granted: true }],
      responseChanges: [
        { role: "moderator", status: "planned", granted: true },
        { role: "moderator", status: "completed", granted: true },
      ],
    },
  }])
})

test("undoing an accepted response preserves a different status granted by another admin", async () => {
  render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  await click(cell("Moderator", "Respond: Planned"))
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Save", exact: true })))
  const server = pageProps(serverPermissions())
  server.permissions.moderator.push("readResponses")
  server.responses.moderator = [PostStatusValue.Planned, PostStatusValue.Completed]
  await act(async () => respond?.({ ok: true, data: server }))

  fireEvent.keyDown(document.body, { key: "z", ctrlKey: true })
  expect(isGranted("Moderator", "Respond: Planned")).toBe(false)
  expect(isGranted("Moderator", "Respond: Completed")).toBe(true)
  expect(isGranted("Moderator", "Use canned responses")).toBe(true)
})

test("a lost response save restores the same status operation and keeps later edits", async () => {
  const put = jest.mocked(http.put)
  put.mockRejectedValueOnce(new RequestError("PUT", "/api/admin/permissions", "transport", new Error("lost acknowledgement")))
  const first = render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  await click(cell("Moderator", "Respond: Planned"))
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Save", exact: true })))
  const operation = put.mock.calls[0][1]
  await click(cell("Helper", "Respond: Duplicate"))
  first.unmount()

  const server = pageProps(serverPermissions())
  server.permissions.moderator.push("readResponses")
  server.responses.moderator = []
  render(<ManagePermissionsPage {...server} />)
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Retry save", exact: true })))
  expect(put.mock.calls[1][1]).toEqual(operation)
  await act(async () => respond?.({ ok: true, data: server }))

  expect(isGranted("Moderator", "Respond: Planned")).toBe(false)
  expect(isGranted("Helper", "Respond: Duplicate")).toBe(true)
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Save", exact: true })))
  expect(put.mock.calls[2][1]).toMatchObject({
    changes: [{ role: "helper", permission: "readResponses", granted: true }],
    responseChanges: [{ role: "helper", status: "duplicate", granted: true }],
  })
})

test("revoking the shared response prerequisite clears statuses and undo restores them", async () => {
  const props = pageProps(serverPermissions())
  props.permissions.moderator.push("readResponses")
  props.responses.moderator = [PostStatusValue.Planned, PostStatusValue.Duplicate]
  render(<ManagePermissionsPage {...props} />)
  await click(cell("Moderator", "Use canned responses"))
  expect(isGranted("Moderator", "Respond: Planned")).toBe(false)
  expect(isGranted("Moderator", "Respond: Duplicate")).toBe(false)

  fireEvent.keyDown(document.body, { key: "z", ctrlKey: true })
  expect(isGranted("Moderator", "Respond: Planned")).toBe(true)
  expect(isGranted("Moderator", "Respond: Duplicate")).toBe(true)
  expect(isGranted("Moderator", "Use canned responses")).toBe(true)
})

test("fresh response locks returned with a rejected save govern the same matrix", async () => {
  render(<ManagePermissionsPage {...pageProps(serverPermissions())} />)
  await click(cell("Moderator", "Respond: Completed"))
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Save", exact: true })))
  const result = {
    ...pageProps(serverPermissions()),
    responseLocks: { moderator: { completed: "This status is not editable" } },
    blocked: "This status is not editable",
  }
  await act(async () => respond?.({ ok: true, data: result }))
  expect(cell("Moderator", "Respond: Completed")).toHaveAttribute("data-tooltip", result.blocked)
  expect(isGranted("Moderator", "Respond: Completed")).toBe(false)
  await click(cell("Moderator", "Respond: Planned"))
  expect(isGranted("Moderator", "Respond: Planned")).toBe(true)
})
