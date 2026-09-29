import React, { memo, useEffect, useMemo, useRef, useState } from "react"
import { AnimatedCount, Dropdown, Icon } from "@fider/components"
import { classSet, notify } from "@fider/services"
import {
  heroiconsCheck as IconCheck,
  heroiconsChevronDown as IconChevronDown,
  heroiconsDotsHorizontal as IconDots,
  heroiconsExclamation as IconExclamation,
} from "@fider/icons.generated"
import { PermissionCell, permissionInfo, ROLE_COLUMNS } from "../catalog"
import { Grants, CellAssignment, CellRequirements, CellLocks } from "../grants"

export interface GridPoint {
  row: number
  col: number
}

export interface GridRange {
  top: number
  left: number
  bottom: number
  right: number
}

// Rows index the visible rows; `anchor` is where shift-extension starts.
export interface GridSelection {
  ranges: GridRange[]
  anchor: GridPoint
  focus: GridPoint
}

export const EMPTY_SELECTION: GridSelection = { ranges: [], anchor: { row: 0, col: 0 }, focus: { row: 0, col: 0 } }

const COLUMN_COUNT = ROLE_COLUMNS.length
const PAGE_ROWS = 10
const AUTOSCROLL_EDGE_PX = 56
const AUTOSCROLL_MAX_PX = 18

const rangeBetween = (a: GridPoint, b: GridPoint): GridRange => ({
  top: Math.min(a.row, b.row),
  bottom: Math.max(a.row, b.row),
  left: Math.min(a.col, b.col),
  right: Math.max(a.col, b.col),
})

const single = (point: GridPoint): GridSelection => ({ ranges: [rangeBetween(point, point)], anchor: point, focus: point })

const selectionMask = (selection: GridSelection, rowCount: number): Uint8Array => {
  const mask = new Uint8Array(rowCount * COLUMN_COUNT)
  for (const range of selection.ranges) {
    for (let row = range.top; row <= Math.min(range.bottom, rowCount - 1); row++) {
      mask.fill(1, row * COLUMN_COUNT + range.left, row * COLUMN_COUNT + range.right + 1)
    }
  }
  return mask
}

export const selectedPoints = (selection: GridSelection, rowCount: number): GridPoint[] => {
  const mask = selectionMask(selection, rowCount)
  const points: GridPoint[] = []
  for (let index = 0; index < mask.length; index++) {
    if (mask[index]) points.push({ row: Math.floor(index / COLUMN_COUNT), col: index % COLUMN_COUNT })
  }
  return points
}

export const assignmentsFor = (points: GridPoint[], rows: PermissionCell[], granted: boolean): CellAssignment[] => {
  return points.map((point) => ({ role: ROLE_COLUMNS[point.col].role, permission: rows[point.row], granted }))
}

export const toggleAssignments = (points: GridPoint[], rows: PermissionCell[], grants: Grants): CellAssignment[] => {
  const allGranted = points.every((point) => grants[ROLE_COLUMNS[point.col].role].has(rows[point.row]))
  return assignmentsFor(points, rows, !allGranted)
}

export interface MatrixGroup {
  id: string
  label: string
  permissions: PermissionCell[]
  rows: PermissionCell[]
  collapsed: boolean
}

interface PermissionMatrixProps {
  groups: MatrixGroup[]
  rows: PermissionCell[]
  grants: Grants
  saved: Grants
  defaults?: Grants
  rules: { cells: PermissionCell[]; requires: CellRequirements; locked: CellLocks }
  selection: GridSelection
  onSelectionChange: (selection: GridSelection) => void
  onApply: (assignments: CellAssignment[]) => void
  onToggleGroup: (groupId: string) => void
}

type DragMode = "cells" | "rows" | "cols"

interface Hit {
  row?: number
  col?: number
  group?: { top: number; bottom: number }
  all?: boolean
}

interface DragSession {
  mode: DragMode
  anchor: GridPoint
  base: GridRange[]
  last: GridPoint
  moved: boolean
  pointerX: number
  pointerY: number
  frame: number
  stop: () => void
}

const readHit = (element: Element | null): Hit | null => {
  const target = element?.closest<HTMLElement>("[data-row],[data-col],[data-group-top],[data-all]")
  if (!target) return null

  const { row, col, groupTop, groupBottom, all } = target.dataset
  return {
    row: row === undefined ? undefined : Number(row),
    col: col === undefined ? undefined : Number(col),
    group: groupTop === undefined ? undefined : { top: Number(groupTop), bottom: Number(groupBottom) },
    all: all !== undefined,
  }
}

export const PermissionMatrix: React.FC<PermissionMatrixProps> = (props) => {
  const { groups, rows, grants, saved, selection, onSelectionChange, onApply } = props
  const tableRef = useRef<HTMLTableElement>(null)
  const dragRef = useRef<DragSession | null>(null)
  const clipboardRef = useRef<boolean[][] | null>(null)
  const pointerTypeRef = useRef("mouse")
  const lastRow = rows.length - 1
  const lastCol = COLUMN_COUNT - 1

  // Window listeners outlive renders.
  const latest = useRef({ rows, grants, onApply, onSelectionChange })
  latest.current = { rows, grants, onApply, onSelectionChange }

  useEffect(() => () => dragRef.current?.stop(), [])

  const mask = useMemo(() => selectionMask(selection, rows.length), [selection, rows.length])

  const focusCell = (point: GridPoint) => {
    const cell = tableRef.current?.querySelector<HTMLElement>(`td[data-row="${point.row}"][data-col="${point.col}"]`)
    cell?.focus({ preventScroll: true })
    cell?.scrollIntoView({ block: "nearest", inline: "nearest" })
  }

  const selectionFor = (mode: DragMode, anchor: GridPoint, point: GridPoint, base: GridRange[]): GridSelection => {
    if (mode === "rows") {
      return {
        ranges: [...base, { top: Math.min(anchor.row, point.row), bottom: Math.max(anchor.row, point.row), left: 0, right: lastCol }],
        anchor: { row: anchor.row, col: 0 },
        focus: { row: point.row, col: 0 },
      }
    }
    if (mode === "cols") {
      return {
        ranges: [...base, { top: 0, bottom: lastRow, left: Math.min(anchor.col, point.col), right: Math.max(anchor.col, point.col) }],
        anchor: { row: 0, col: anchor.col },
        focus: { row: 0, col: point.col },
      }
    }
    return { ranges: [...base, rangeBetween(anchor, point)], anchor, focus: point }
  }

  const extendDrag = (session: DragSession) => {
    const hit = readHit(document.elementFromPoint(session.pointerX, session.pointerY))
    if (!hit) return

    const point = {
      row: session.mode === "cols" ? session.last.row : Math.max(0, Math.min(hit.row ?? session.last.row, lastRow)),
      col: session.mode === "rows" ? session.last.col : Math.max(0, Math.min(hit.col ?? session.last.col, lastCol)),
    }
    if (point.row === session.last.row && point.col === session.last.col) return

    session.last = point
    session.moved = true
    latest.current.onSelectionChange(selectionFor(session.mode, session.anchor, point, session.base))
  }

  const startDrag = (mode: DragMode, anchor: GridPoint, point: GridPoint, base: GridRange[], event: React.PointerEvent) => {
    dragRef.current?.stop()
    const toggleOnClick = mode === "cells" && !event.ctrlKey && !event.metaKey && !event.shiftKey

    const onMove = (moveEvent: PointerEvent) => {
      session.pointerX = moveEvent.clientX
      session.pointerY = moveEvent.clientY
      extendDrag(session)
    }

    const autoscroll = () => {
      const { pointerY } = session
      let delta = 0
      if (pointerY < AUTOSCROLL_EDGE_PX) {
        delta = -Math.ceil(((AUTOSCROLL_EDGE_PX - pointerY) / AUTOSCROLL_EDGE_PX) * AUTOSCROLL_MAX_PX)
      } else if (pointerY > window.innerHeight - AUTOSCROLL_EDGE_PX) {
        delta = Math.ceil(((pointerY - (window.innerHeight - AUTOSCROLL_EDGE_PX)) / AUTOSCROLL_EDGE_PX) * AUTOSCROLL_MAX_PX)
      }
      if (delta !== 0) {
        window.scrollBy(0, delta)
        extendDrag(session)
      }
      session.frame = requestAnimationFrame(autoscroll)
    }

    const stop = () => {
      cancelAnimationFrame(session.frame)
      window.removeEventListener("pointermove", onMove)
      window.removeEventListener("pointerup", onUp)
      window.removeEventListener("pointercancel", stop)
      dragRef.current = null
    }

    const onUp = () => {
      const clicked = !session.moved && toggleOnClick
      stop()
      if (clicked) {
        const { rows, grants, onApply } = latest.current
        onApply(toggleAssignments([anchor], rows, grants))
      }
    }

    const session: DragSession = { mode, anchor, base, last: point, moved: false, pointerX: event.clientX, pointerY: event.clientY, frame: 0, stop }
    dragRef.current = session
    session.frame = requestAnimationFrame(autoscroll)
    window.addEventListener("pointermove", onMove)
    window.addEventListener("pointerup", onUp)
    window.addEventListener("pointercancel", stop)
  }

  // Touch never drags so it keeps native scrolling.
  const press = (hit: Hit, event: React.PointerEvent | React.MouseEvent, canDrag: boolean) => {
    const additive = event.ctrlKey || event.metaKey
    const extend = event.shiftKey && selection.ranges.length > 0
    const base = additive ? selection.ranges : extend ? selection.ranges.slice(0, -1) : []

    if (hit.all) {
      if (rows.length > 0)
        onSelectionChange({ ranges: [{ top: 0, bottom: lastRow, left: 0, right: lastCol }], anchor: { row: 0, col: 0 }, focus: selection.focus })
      return
    }

    if (hit.group) {
      if (hit.group.top > hit.group.bottom) return
      const range = { top: hit.group.top, bottom: hit.group.bottom, left: hit.col ?? 0, right: hit.col ?? lastCol }
      const anchor = { row: range.top, col: range.left }
      onSelectionChange({ ranges: [...(additive ? selection.ranges : []), range], anchor, focus: anchor })
      focusCell(anchor)
      return
    }

    if (rows.length === 0) return

    let mode: DragMode = "cells"
    if (hit.row === undefined) mode = "cols"
    else if (hit.col === undefined) mode = "rows"

    const point = { row: hit.row ?? 0, col: hit.col ?? 0 }
    const anchor = extend ? selection.anchor : point
    const next = selectionFor(mode, anchor, point, base)
    onSelectionChange(next)
    focusCell(next.focus)

    if (canDrag) {
      startDrag(mode, anchor, point, base, event as React.PointerEvent)
    } else if (mode === "cells" && !additive && !event.shiftKey) {
      onApply(toggleAssignments([point], rows, grants))
    }
  }

  const onPointerDown = (event: React.PointerEvent) => {
    pointerTypeRef.current = event.pointerType
    if (event.button !== 0 || event.pointerType === "touch") return
    if ((event.target as HTMLElement).closest("[data-no-select]")) return

    const hit = readHit(event.target as Element)
    if (!hit) return

    event.preventDefault()
    press(hit, event, !hit.all && !hit.group)
  }

  const onClick = (event: React.MouseEvent) => {
    if (pointerTypeRef.current !== "touch" || (event.target as HTMLElement).closest("[data-no-select]")) return
    const hit = readHit(event.target as Element)
    if (hit) press(hit, event, false)
  }

  const activePoints = (): GridPoint[] => {
    const points = selectedPoints(selection, rows.length)
    return points.length > 0 ? points : [selection.focus]
  }

  const copy = () => {
    const range = selection.ranges[selection.ranges.length - 1] ?? rangeBetween(selection.focus, selection.focus)
    const values: boolean[][] = []
    for (let row = range.top; row <= range.bottom; row++) {
      const line: boolean[] = []
      for (let col = range.left; col <= range.right; col++) {
        line.push(grants[ROLE_COLUMNS[col].role].has(rows[row]))
      }
      values.push(line)
    }
    clipboardRef.current = values
    notify.success(`Copied ${values.length} × ${values[0].length}`)
  }

  const paste = () => {
    const values = clipboardRef.current
    if (!values) return

    const height = values.length
    const width = values[0].length
    const selected = selection.ranges[selection.ranges.length - 1]
    const tiles = selected && (selected.bottom - selected.top + 1) % height === 0 && (selected.right - selected.left + 1) % width === 0
    const target = tiles
      ? selected
      : {
          top: selection.focus.row,
          left: selection.focus.col,
          bottom: Math.min(selection.focus.row + height - 1, lastRow),
          right: Math.min(selection.focus.col + width - 1, lastCol),
        }

    const assignments: CellAssignment[] = []
    for (let row = target.top; row <= target.bottom; row++) {
      for (let col = target.left; col <= target.right; col++) {
        assignments.push({
          role: ROLE_COLUMNS[col].role,
          permission: rows[row],
          granted: values[(row - target.top) % height][(col - target.left) % width],
        })
      }
    }
    onApply(assignments)
    onSelectionChange({ ranges: [target], anchor: { row: target.top, col: target.left }, focus: selection.focus })
  }

  const onKeyDown = (event: React.KeyboardEvent) => {
    if (rows.length === 0 || (event.target as HTMLElement).closest("[data-no-select]")) return

    const mod = event.ctrlKey || event.metaKey
    const { focus } = selection
    let next: GridPoint | null = null

    switch (event.key) {
      case "ArrowUp":
        next = { row: mod ? 0 : focus.row - 1, col: focus.col }
        break
      case "ArrowDown":
        next = { row: mod ? lastRow : focus.row + 1, col: focus.col }
        break
      case "ArrowLeft":
        next = { row: focus.row, col: mod ? 0 : focus.col - 1 }
        break
      case "ArrowRight":
        next = { row: focus.row, col: mod ? lastCol : focus.col + 1 }
        break
      case "Home":
        next = { row: mod ? 0 : focus.row, col: 0 }
        break
      case "End":
        next = { row: mod ? lastRow : focus.row, col: lastCol }
        break
      case "PageUp":
        next = { row: focus.row - PAGE_ROWS, col: focus.col }
        break
      case "PageDown":
        next = { row: focus.row + PAGE_ROWS, col: focus.col }
        break
    }

    if (next) {
      event.preventDefault()
      next = { row: Math.max(0, Math.min(next.row, lastRow)), col: Math.max(0, Math.min(next.col, lastCol)) }
      if (event.shiftKey) {
        onSelectionChange({ ranges: [...selection.ranges.slice(0, -1), rangeBetween(selection.anchor, next)], anchor: selection.anchor, focus: next })
      } else {
        onSelectionChange(single(next))
      }
      focusCell(next)
      return
    }

    const key = event.key.toLowerCase()
    if (mod && key === "a") {
      onSelectionChange({ ranges: [{ top: 0, bottom: lastRow, left: 0, right: lastCol }], anchor: { row: 0, col: 0 }, focus })
    } else if (mod && key === "c") {
      copy()
    } else if (mod && key === "v") {
      paste()
    } else if (mod || event.altKey) {
      return
    } else if (key === " " || key === "enter") {
      onApply(toggleAssignments(activePoints(), rows, grants))
    } else if (key === "g" || key === "+") {
      onApply(assignmentsFor(activePoints(), rows, true))
    } else if (key === "r" || key === "-") {
      onApply(assignmentsFor(activePoints(), rows, false))
    } else if (key === "escape" && selection.ranges.length > 0) {
      onSelectionChange({ ...selection, ranges: [] })
    } else {
      return
    }
    event.preventDefault()
  }

  const columnAction = (col: number, assignments: CellAssignment[]) => {
    onApply(assignments)
    onSelectionChange(selectionFor("cols", { row: 0, col }, { row: 0, col }, []))
  }

  const filtered = rows.length < props.rules.cells.length
  let rowIndex = 0

  return (
    <table
      ref={tableRef}
      role="grid"
      aria-label="Role permissions"
      aria-multiselectable="true"
      className="w-full border-separate border-spacing-0 text-left select-none"
      onPointerDown={onPointerDown}
      onClick={onClick}
      onKeyDown={onKeyDown}
    >
      <thead>
        <tr>
          <th
            scope="col"
            data-all
            className="sticky top-0 max-lg:left-0 z-20 bg-elevated border-b border-surface-alt px-3 py-2 text-xs font-semibold uppercase tracking-wide text-muted cursor-pointer"
          >
            Permission
          </th>
          {ROLE_COLUMNS.map((column, col) => {
            const role = column.role
            const others = ROLE_COLUMNS.filter((other) => other.role !== role)
            const columnPoints = rows.map((_, row) => ({ row, col }))
            const defaults = props.defaults
            const editable = props.rules.cells.some((permission) => !props.rules.locked[role]?.[permission])
            return (
              <th
                key={role}
                scope="col"
                data-col={col}
                className="group/col sticky top-0 z-10 w-24 min-w-24 bg-elevated border-b border-surface-alt px-1 py-2 font-normal cursor-pointer"
              >
                <div className="flex flex-col items-center leading-tight">
                  <span className={`text-sm font-semibold ${column.className}`}>{column.label}</span>
                  <div className={classSet({ "flex items-center gap-0.5": true, "pl-4": editable })}>
                    <span className="text-[11px] text-muted tabular-nums">
                      <AnimatedCount value={grants[role].size} />/{props.rules.cells.length}
                    </span>
                    {editable && (
                      <div data-no-select className="flex">
                        <Dropdown
                          position="left"
                          label={`${column.label} actions`}
                          renderHandle={
                            <span className="flex rounded-badge px-0.5 text-subtle transition-colors duration-100 hover:bg-tertiary hover:text-foreground">
                              <Icon sprite={IconDots} className="h-4 w-4" />
                            </span>
                          }
                        >
                          <Dropdown.ListItem onClick={() => columnAction(col, assignmentsFor(columnPoints, rows, true))}>
                            {filtered ? "Grant shown editable" : "Grant editable"}
                          </Dropdown.ListItem>
                          <Dropdown.ListItem onClick={() => columnAction(col, assignmentsFor(columnPoints, rows, false))}>
                            {filtered ? "Revoke shown editable" : "Revoke editable"}
                          </Dropdown.ListItem>
                          <Dropdown.Divider />
                          {others.map((other) => (
                            <Dropdown.ListItem
                              key={other.role}
                              onClick={() =>
                                columnAction(
                                  col,
                                  props.rules.cells.map((permission) => ({ role, permission, granted: grants[other.role].has(permission) }))
                                )
                              }
                            >
                              Copy editable from <span className={other.className}>{other.label}</span>
                            </Dropdown.ListItem>
                          ))}
                          {defaults && (
                            <>
                              <Dropdown.Divider />
                              <Dropdown.ListItem
                                onClick={() =>
                                  columnAction(
                                    col,
                                    props.rules.cells.map((permission) => ({ role, permission, granted: defaults[role].has(permission) }))
                                  )
                                }
                              >
                                Reset editable to default
                              </Dropdown.ListItem>
                            </>
                          )}
                        </Dropdown>
                      </div>
                    )}
                  </div>
                </div>
              </th>
            )
          })}
        </tr>
      </thead>
      {groups.map((group) => {
        const top = rowIndex
        const bottom = rowIndex + group.rows.length - 1
        return (
          <tbody key={group.id}>
            <tr>
              <th
                scope="rowgroup"
                colSpan={COLUMN_COUNT + 1}
                data-group-top={top}
                data-group-bottom={bottom}
                className="border-b border-border pt-4 pb-1.5 px-3 cursor-pointer"
              >
                <div className="flex w-max items-center gap-1.5 max-lg:sticky max-lg:left-3">
                  <button
                    type="button"
                    data-no-select
                    aria-expanded={!group.collapsed}
                    aria-label={`${group.collapsed ? "Expand" : "Collapse"} ${group.label}`}
                    onClick={() => props.onToggleGroup(group.id)}
                    className="-ml-1 flex rounded-badge p-0.5 text-muted transition-colors duration-100 hover:bg-tertiary hover:text-foreground"
                  >
                    <Icon
                      sprite={IconChevronDown}
                      className={classSet({ "h-4 w-4 transition-transform duration-150 ease-out": true, "-rotate-90": group.collapsed })}
                    />
                  </button>
                  <span className="text-category text-xs tracking-wide">{group.label}</span>
                </div>
              </th>
            </tr>
            {group.rows.map((permission) => {
              const row = rowIndex++
              const info = permissionInfo(permission)
              const rowSelected = mask.subarray(row * COLUMN_COUNT, (row + 1) * COLUMN_COUNT).every(Boolean)
              const requires = props.rules.requires[permission]?.map((requirement) => permissionInfo(requirement).label).join(", ")
              return (
                <tr key={permission} className="group/row">
                  <th
                    scope="row"
                    data-row={row}
                    className={classSet({
                      "max-lg:sticky max-lg:left-0 max-lg:z-[1] max-lg:w-40 max-lg:min-w-40 max-lg:max-w-40 border-b border-surface-alt px-3 py-1.5 font-normal cursor-pointer transition-colors duration-100": true,
                      "max-lg:bg-surface group-hover/row:bg-surface-alt": !rowSelected,
                      "bg-primary/15": rowSelected,
                    })}
                  >
                    <div className="flex items-center gap-1.5 text-sm text-foreground">
                      {info.label}
                      {info.sensitive && (
                        <span data-tooltip="Sensitive" className="flex text-warning">
                          <Icon sprite={IconExclamation} className="h-3.5 w-3.5" />
                        </span>
                      )}
                    </div>
                    <div className="max-w-[22rem] max-lg:max-w-full truncate text-xs text-muted" title={info.description}>
                      {info.description}
                      {requires && <span className="text-subtle">; needs {requires}</span>}
                    </div>
                  </th>
                  {ROLE_COLUMNS.map((column, col) => (
                    <MatrixCell
                      key={column.role}
                      row={row}
                      col={col}
                      label={`${column.label}: ${info.label}`}
                      granted={grants[column.role].has(permission)}
                      wasGranted={saved[column.role].has(permission)}
                      lock={props.rules.locked[column.role]?.[permission]}
                      selected={mask[row * COLUMN_COUNT + col] === 1}
                      focused={selection.focus.row === row && selection.focus.col === col}
                    />
                  ))}
                </tr>
              )
            })}
          </tbody>
        )
      })}
    </table>
  )
}

interface MatrixCellProps {
  row: number
  col: number
  label: string
  granted: boolean
  wasGranted: boolean
  lock?: string
  selected: boolean
  focused: boolean
}

const MatrixCell = memo(function MatrixCell({ row, col, label, granted, wasGranted, lock, selected, focused }: MatrixCellProps) {
  // Initial grants must not animate.
  const [shown, setShown] = useState({ granted, animate: false })
  if (shown.granted !== granted) {
    setShown({ granted, animate: granted })
  }

  const changed = granted !== wasGranted
  let tooltip: string | undefined
  if (lock) {
    tooltip = lock
  } else if (changed) {
    tooltip = wasGranted ? "Unsaved; was granted" : "Unsaved; was not granted"
  }

  return (
    <td
      role="gridcell"
      data-row={row}
      data-col={col}
      tabIndex={focused ? 0 : -1}
      aria-selected={selected}
      aria-label={`${label}, ${granted ? "granted" : "not granted"}${lock ? ", locked" : ""}${changed ? ", unsaved" : ""}`}
      data-tooltip={tooltip}
      className={classSet({
        "group/cell relative border-b border-surface-alt text-center align-middle outline-none scroll-mt-16 scroll-mb-24 transition-colors duration-100 ease-out focus-visible:shadow-[inset_0_0_0_2px_var(--color-primary)]": true,
        "group-hover/row:bg-surface-alt": !selected,
        "bg-primary/15": selected,
        "cursor-pointer": !lock,
        "cursor-not-allowed": !!lock,
      })}
    >
      <span
        className={classSet({
          "relative inline-flex h-5 w-5 items-center justify-center rounded-badge border align-middle transition-[background-color,border-color,transform] duration-100 ease-out group-active/cell:scale-90": true,
          "border-primary bg-primary text-surface": granted && !lock,
          "border-accent-medium bg-accent-medium text-muted": granted && !!lock,
          "border-border-strong group-hover/cell:border-primary": !granted && !lock,
          "border-dashed border-border opacity-60": !granted && !!lock,
        })}
      >
        {granted && <Icon sprite={IconCheck} className={shown.animate ? "h-3.5 w-3.5 wipe-check" : "h-3.5 w-3.5"} />}
        {changed && <span aria-hidden="true" className="absolute -top-1 -right-1 h-2 w-2 rounded-full border border-surface bg-warning" />}
      </span>
    </td>
  )
})
