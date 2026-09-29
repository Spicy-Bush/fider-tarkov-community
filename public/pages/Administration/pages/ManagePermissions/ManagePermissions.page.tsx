import React, { useEffect, useMemo, useReducer, useRef, useState } from "react"
import { Button, Icon } from "@fider/components"
import { PageConfig } from "@fider/components/layouts"
import { useFider } from "@fider/hooks"
import { RolePermissions, SavedRolePermissions } from "@fider/models"
import { actions, classSet, notify, tryLocalStorageGet, tryLocalStorageRemove, tryLocalStorageSet, tryParseJSON } from "@fider/services"
import { RequestError, requestOutcome } from "@fider/services/http"
import { newSubmissionID } from "@fider/services/postSubmission"
import { heroiconsSearch as IconSearch, heroiconsX as IconX } from "@fider/icons.generated"
import { PermissionCell, permissionInfo, permissionGroups, ROLE_COLUMNS } from "./catalog"
import { Grants, CellAssignment, PermissionGraph, diffGrants, editableChoices, serverPermissionGraph, permissionLocks, preserveOtherGrants, projectAssignments, saveAssignments, showChoices, toGrants } from "./grants"
import { EMPTY_SELECTION, GridSelection, MatrixGroup, PermissionMatrix, assignmentsFor, selectedPoints } from "./components/PermissionMatrix"

export const pageConfig: PageConfig = {
  title: "Permissions",
  subtitle: "Choose what each role can do",
  sidebarItem: "permissions",
}

interface ManagePermissionsPageProps extends SavedRolePermissions {
  defaults: RolePermissions
}

const HISTORY_LIMIT = 100

interface EditorState {
  rules: PermissionGraph
  saved: Grants
  present: Grants
  past: CellAssignment[][]
  future: CellAssignment[][]
  blocked?: string
}

type EditorAction =
  | { type: "apply"; assignments: CellAssignment[] }
  | { type: "undo" }
  | { type: "redo" }
  | { type: "discard" }
  | { type: "saved"; result: SavedRolePermissions; sent: Grants }

const editorReducer = (state: EditorState, action: EditorAction): EditorState => {
  switch (action.type) {
    case "apply":
    case "undo":
    case "redo": {
      let assignments: CellAssignment[]
      if (action.type === "apply") {
        assignments = action.assignments
      } else if (action.type === "undo") {
        if (state.past.length === 0) {
          return state
        }

        assignments = state.past[state.past.length - 1].map((change) => ({ ...change, granted: !change.granted }))
      } else {
        if (state.future.length === 0) {
          return state
        }

        assignments = state.future[0]
      }

      if (action.type !== "apply") {
        assignments = preserveOtherGrants(state.present, state.rules, assignments)
      }
      const projected = projectAssignments(state.present, state.rules, assignments)
      if (projected.blocked) {
        return { ...state, blocked: projected.blocked }
      }

      const edit = diffGrants(state.present, projected.grants)

      let past = state.past
      let future = state.future
      if (action.type === "undo") {
        future = [past[past.length - 1], ...future]
        past = past.slice(0, -1)
      } else if (action.type === "redo") {
        past = [...past, edit].slice(-HISTORY_LIMIT)
        future = future.slice(1)
      } else if (edit.length > 0) {
        past = [...past, edit].slice(-HISTORY_LIMIT)
        future = []
      }

      return {
        ...state,
        present: projected.grants,
        past,
        future,
        blocked: undefined,
      }
    }
    case "discard": {
      const edit = diffGrants(state.present, state.saved)

      return {
        ...state,
        present: state.saved,
        past: edit.length > 0 ? [...state.past, edit].slice(-HISTORY_LIMIT) : state.past,
        future: [],
        blocked: undefined,
      }
    }
    case "saved": {
      const server = toGrants(action.result.permissions, action.result.responses)
      const rules = serverPermissionGraph(action.result)
      const rejected = !!action.result.blocked
      const localChanges = diffGrants(rejected ? state.saved : action.sent, state.present)
      const remaining = editableChoices(localChanges, permissionLocks(server, rules))
      const projected = projectAssignments(server, rules, remaining)

      return {
        ...state,
        rules,
        saved: server,
        present: projected.grants,
        blocked: action.result.blocked || projected.blocked,
      }
    }

  }
}

const KNOWN_ROLES = new Set<string>(ROLE_COLUMNS.map((column) => column.role))

const draftKey = (userID: number) => `fider:permissions-draft:${userID}`

// Store only edited cells so restoration preserves unrelated changes by other admins.
interface PermissionDraft {
  changes: CellAssignment[]
  pending?: { submissionId: string; changes: CellAssignment[] }
}

const readDraft = (userID: number, cells: PermissionCell[]): PermissionDraft | undefined => {
  const stored = tryParseJSON<Partial<PermissionDraft>>(tryLocalStorageGet(draftKey(userID)) ?? "{}", {})
  if (!stored || !Array.isArray(stored.changes)) {
    return undefined
  }

  const knownCells = new Set<string>(cells)
  const validAssignment = (entry: unknown): entry is CellAssignment =>
    typeof entry === "object" &&
    entry !== null &&
    "role" in entry && typeof entry.role === "string" && KNOWN_ROLES.has(entry.role) &&
    "permission" in entry && typeof entry.permission === "string" && knownCells.has(entry.permission) &&
    "granted" in entry && typeof entry.granted === "boolean"

  const changes = stored.changes.filter(validAssignment)
  const pending = stored.pending && Array.isArray(stored.pending.changes) && typeof stored.pending.submissionId === "string"
    ? { submissionId: stored.pending.submissionId, changes: stored.pending.changes.filter(validAssignment) }
    : undefined

  return { changes, pending }
}

const matches = (permission: PermissionCell, query: string): boolean => {
  const info = permissionInfo(permission)
  return info.label.toLowerCase().includes(query) || info.description.toLowerCase().includes(query) || permission.toLowerCase().includes(query)
}

const plural = (count: number, word: string) => `${count} ${word}${count === 1 ? "" : "s"}`

const ManagePermissionsPage: React.FC<ManagePermissionsPageProps> = (props) => {
  const fider = useFider()
  const userID = fider.session.user.id
  const defaults = useMemo(() => toGrants(props.defaults, props.defaultResponses), [props.defaults, props.defaultResponses])

  const [initial] = useState(() => {
    const rules = serverPermissionGraph(props)
    const saved = toGrants(props.permissions, props.responses)
    const draft = readDraft(userID, rules.cells)
    const restored = draft?.pending && { ...draft.pending, sent: showChoices(saved, draft.pending.changes), uncertain: true }
    const intended = restored?.sent ?? saved
    const assignments = editableChoices(draft?.changes ?? [], permissionLocks(intended, rules))
    const projected = projectAssignments(intended, rules, assignments)
    const present = projected.grants
    return {
      state: {
        rules,
        saved,
        present,
        past: [],
        future: [],
        blocked: projected.blocked,
      } as EditorState,
      restored: diffGrants(saved, present).length,
      pending: restored,
    }
  })
  const [state, dispatch] = useReducer(editorReducer, initial.state)
  const [restoredCount, setRestoredCount] = useState(initial.restored)
  const present = state.present
  const locks = useMemo(() => permissionLocks(present, state.rules), [present, state.rules])
  const changes = useMemo(() => diffGrants(state.saved, present), [state.saved, present])

  const [saving, setSaving] = useState(false)
  const [pendingSave, setPendingSave] = useState(initial.pending)
  const [storageFailed, setStorageFailed] = useState(false)
  const [query, setQuery] = useState("")
  const [changedFilter, setChangedFilter] = useState<ReadonlySet<PermissionCell> | null>(null)
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(new Set())
  const searchRef = useRef<HTMLInputElement>(null)

  const storeDraft = () => {
    const draft: PermissionDraft = {
      changes: diffGrants(pendingSave?.sent ?? state.saved, state.present),
      pending: pendingSave && { submissionId: pendingSave.submissionId, changes: pendingSave.changes },
    }
    const stored = changes.length === 0 && !pendingSave
      ? tryLocalStorageRemove(draftKey(userID))
      : tryLocalStorageSet(draftKey(userID), JSON.stringify(draft))
    setStorageFailed(!stored)
  }

  useEffect(() => {
    storeDraft()
    if (changes.length === 0 && !pendingSave) {
      return
    }

    const beforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault()
      event.returnValue = ""
    }
    window.addEventListener("beforeunload", beforeUnload)
    return () => window.removeEventListener("beforeunload", beforeUnload)
  }, [changes, state.present, pendingSave, userID])

  const { groups, rows } = useMemo(() => {
    const search = query.trim().toLowerCase()
    const groups: MatrixGroup[] = []
    for (const group of permissionGroups(state.rules.cells)) {
      const matching = group.permissions.filter((permission) => (!changedFilter || changedFilter.has(permission)) && (!search || matches(permission, search)))
      if (matching.length === 0) continue

      const isCollapsed = collapsed.has(group.id) && !search && !changedFilter
      groups.push({ ...group, rows: isCollapsed ? [] : matching, collapsed: isCollapsed })
    }
    return { groups, rows: groups.flatMap((group) => group.rows) }
  }, [query, changedFilter, collapsed, state.rules.cells])

  // Selection indexes visible rows, so it resets when they change.
  const [selectionState, setSelectionState] = useState({ rows, selection: EMPTY_SELECTION })
  const selection = selectionState.rows === rows ? selectionState.selection : EMPTY_SELECTION
  const setSelection = (next: GridSelection) => setSelectionState({ rows, selection: next })
  const selected = useMemo(() => selectedPoints(selection, rows.length), [selection, rows.length])

  const apply = (assignments: CellAssignment[]) => {
    const editable = editableChoices(assignments, locks)
    if (editable.length > 0) {
      dispatch({ type: "apply", assignments: editable })
    }
  }

  const save = async () => {
    const sent = pendingSave?.sent ?? state.present
    const sentChanges = pendingSave?.changes ?? diffGrants(state.saved, sent)
    if (sentChanges.length === 0 || saving) {
      return
    }

    const operation = pendingSave ?? { submissionId: newSubmissionID(), sent, changes: sentChanges, uncertain: false }
    setPendingSave(operation)
    setSaving(true)

    try {
      const wire = saveAssignments(sentChanges)
      const result = await actions.updateRolePermissions(operation.submissionId, wire.changes, wire.responseChanges)
      if (result.ok) {
        setPendingSave(undefined)
        dispatch({ type: "saved", result: result.data, sent })
        setRestoredCount(0)
        if (!result.data.blocked) {
          notify.success(`Saved ${plural(sentChanges.length, "change")}`)
        }
      } else if (requestOutcome(result) === "rejected" && !operation.uncertain) {
        setPendingSave(undefined)
        notify.error(result.error.errors?.[0]?.message || "Permissions could not be saved.")
      } else {
        setPendingSave({ ...operation, uncertain: true })
      }
    } catch (cause) {
      if (!(cause instanceof RequestError)) throw cause
      setPendingSave({ ...operation, uncertain: true })
      notify.error("Permissions could not be saved. Your changes are kept; try again.")
    } finally {
      setSaving(false)
    }
  }

  const saveRef = useRef(save)
  saveRef.current = save

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement
      const typing = target.closest("input, textarea, select, [contenteditable=true]")
      const mod = event.ctrlKey || event.metaKey
      const key = event.key.toLowerCase()

      if (mod && key === "s") {
        event.preventDefault()
        saveRef.current()
      } else if (typing) {
        return
      } else if (mod && key === "z") {
        event.preventDefault()
        dispatch({ type: event.shiftKey ? "redo" : "undo" })
      } else if (mod && key === "y") {
        event.preventDefault()
        dispatch({ type: "redo" })
      } else if (key === "/" && !mod) {
        event.preventDefault()
        searchRef.current?.focus()
      }
    }

    document.addEventListener("keydown", onKeyDown)
    return () => document.removeEventListener("keydown", onKeyDown)
  }, [])

  const toggleChangedFilter = () => {
    // Frozen so reverted rows don't vanish mid-edit.
    setChangedFilter(changedFilter ? null : new Set(changes.map((change) => change.permission)))
  }

  const toggleGroup = (groupId: string) => {
    const next = new Set(collapsed)
    if (next.has(groupId)) next.delete(groupId)
    else next.add(groupId)
    setCollapsed(next)
  }

  const showSelectionActions = selected.length > 1
  const showBar = showSelectionActions || changes.length > 0 || !!pendingSave

  return (
    <div className="pb-28">
      {state.blocked && <p role="alert" className="mb-4 text-sm text-danger">{state.blocked}</p>}
      {storageFailed && (
        <div role="alert" className="mb-4 flex flex-wrap items-center gap-3 text-sm text-danger">
          <span>
            {changes.length > 0
              ? "Browser draft storage is unavailable. Save your changes before leaving this page."
              : "Browser draft cleanup failed. Retry to clear any saved copy."}
          </span>
          <Button size="small" onClick={storeDraft}>
            Retry draft storage
          </Button>
        </div>
      )}
      {restoredCount > 0 && changes.length > 0 && (
        <div
          role="status"
          className="mb-4 flex flex-wrap items-center gap-3 rounded-card border border-warning-medium bg-warning-light px-4 py-2.5 text-sm popover-enter"
        >
          <span className="flex-1">Restored {plural(restoredCount, "unsaved change")} from your last visit.</span>
          <Button size="small" variant="tertiary" onClick={() => setRestoredCount(0)}>
            Dismiss
          </Button>
        </div>
      )}

      <div className="mb-4 flex flex-wrap items-center gap-2">
        <div className="relative min-w-48 flex-1 max-w-80">
          <Icon sprite={IconSearch} className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-subtle" />
          <input
            ref={searchRef}
            type="search"
            value={query}
            aria-label="Filter permissions"
            placeholder="Filter permissions"
            onChange={(event) => setQuery(event.currentTarget.value)}
            onKeyDown={(event) => {
              if (event.key === "Escape") {
                setQuery("")
                event.currentTarget.blur()
              }
            }}
            className="h-10 w-full rounded-input border border-border bg-elevated pl-8 pr-3 text-sm text-foreground outline-none transition-colors duration-100 placeholder:text-subtle focus:border-primary"
          />
        </div>
        <button
          type="button"
          aria-pressed={!!changedFilter}
          disabled={!changedFilter && changes.length === 0}
          onClick={toggleChangedFilter}
          className={classSet({
            "flex items-center h-10 text-xs font-medium uppercase rounded-button border px-3 py-2 cursor-pointer transition-colors bg-transparent disabled:cursor-not-allowed disabled:opacity-50": true,
            "border-border text-foreground hover:bg-tertiary hover:border-border-strong": !changedFilter,
            "border-primary text-primary": !!changedFilter,
          })}
        >
          Unsaved
          {changes.length > 0 && (
            <span className="bg-primary text-white inline-block rounded-full px-2 py-0.5 min-w-[20px] text-[10px] text-center ml-2">{changes.length}</span>
          )}
        </button>
        <div className="ml-auto flex items-center gap-1">
          <Button size="small" variant="secondary" disabled={state.past.length === 0} onClick={() => dispatch({ type: "undo" })}>
            Undo
          </Button>
          <Button size="small" variant="secondary" disabled={state.future.length === 0} onClick={() => dispatch({ type: "redo" })}>
            Redo
          </Button>
        </div>
      </div>

      {groups.length === 0 ? (
        <p className="py-12 text-center text-sm text-muted">No permissions match.</p>
      ) : (
        <div className="max-lg:overflow-x-auto">
          <PermissionMatrix
            groups={groups}
            rows={rows}
            grants={present}
            saved={state.saved}
            defaults={defaults}
            rules={{ cells: state.rules.cells, requires: state.rules.requires, locked: locks }}
            selection={selection}
            onSelectionChange={setSelection}
            onApply={apply}
            onToggleGroup={toggleGroup}
          />
        </div>
      )}

      {showBar && (
        <div className="fixed bottom-4 left-1/2 z-40 w-max max-w-[calc(100vw-2rem)] -translate-x-1/2 max-md:bottom-20">
          <div className="flex flex-wrap items-center gap-x-4 gap-y-2 rounded-card border border-border-strong bg-elevated px-4 py-2.5 shadow-xl popover-enter">
            {showSelectionActions && (
              <div className="flex items-center gap-2">
                <span className="text-sm tabular-nums text-muted">{selected.length} selected</span>
                <Button size="small" variant="secondary" onClick={() => apply(assignmentsFor(selected, rows, true))}>
                  Grant editable
                </Button>
                <Button size="small" variant="secondary" onClick={() => apply(assignmentsFor(selected, rows, false))}>
                  Revoke editable
                </Button>
                <button
                  type="button"
                  aria-label="Clear selection"
                  onClick={() => setSelection({ ...selection, ranges: [] })}
                  className="flex rounded-badge p-1 text-subtle transition-colors duration-100 hover:bg-tertiary hover:text-foreground"
                >
                  <Icon sprite={IconX} className="h-4 w-4" />
                </button>
              </div>
            )}
            {showSelectionActions && changes.length > 0 && <span aria-hidden="true" className="h-5 w-px bg-border max-sm:hidden" />}
            {(changes.length > 0 || pendingSave) && (
              <div className="flex items-center gap-2">
                <span className="flex items-center gap-1.5 text-sm tabular-nums">
                  <span aria-hidden="true" className="h-2 w-2 rounded-full bg-warning" />
                  {pendingSave ? "Save not confirmed" : plural(changes.length, "unsaved change")}
                </span>
                <Button size="small" variant="tertiary" disabled={saving} onClick={() => {
                  dispatch({ type: "discard" })
                }}>
                  Discard
                </Button>
                <Button size="small" variant="primary" loading={saving} onClick={save}>
                  {pendingSave ? "Retry save" : "Save"}
                </Button>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

export default ManagePermissionsPage
