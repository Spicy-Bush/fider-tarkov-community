import { Permission, PermissionAssignment, ResponsePermissionAssignment, RolePermissions, RoleResponsePermissions, SavedRolePermissions, UserRole } from "@fider/models"
import { ALL_PERMISSIONS, ROLE_COLUMNS, PermissionCell, responseCell, responseStatus } from "./catalog"

export interface CellAssignment {
  role: UserRole
  permission: PermissionCell
  granted: boolean
}

export type CellRequirements = Partial<Record<PermissionCell, PermissionCell[]>>
export type CellLocks = Partial<Record<UserRole, Partial<Record<PermissionCell, string>>>>

export type Grants = Record<UserRole, ReadonlySet<PermissionCell>>

export const toGrants = (lists: RolePermissions, responses?: RoleResponsePermissions): Grants => {
  const grants = {} as Record<UserRole, ReadonlySet<PermissionCell>>
  for (const { role } of ROLE_COLUMNS) {
    grants[role] = new Set([...(lists[role] ?? []), ...(responses?.[role] ?? []).map(responseCell)])
  }
  return grants
}

export const editableChoices = (assignments: CellAssignment[], locked: CellLocks): CellAssignment[] => {
  return assignments.filter(({ role, permission }) => !locked[role]?.[permission])
}

export const toLists = (grants: Grants): Record<UserRole, PermissionCell[]> => {
  return Object.fromEntries(ROLE_COLUMNS.map(({ role }) => [role, [...grants[role]]])) as Record<UserRole, PermissionCell[]>
}

export const showChoices = (grants: Grants, assignments: readonly CellAssignment[]): Grants => {
  const copies = new Map<UserRole, Set<PermissionCell>>()
  for (const { role, permission, granted } of assignments) {
    let choices = copies.get(role)
    if (!choices) {
      choices = new Set(grants[role])
      copies.set(role, choices)
    }

    if (granted) {
      choices.add(permission)
    } else {
      choices.delete(permission)
    }
  }
  return { ...grants, ...Object.fromEntries(copies) }
}

export const diffGrants = (from: Grants, to: Grants): CellAssignment[] => {
  const changes: CellAssignment[] = []
  const cells = new Set<PermissionCell>(ALL_PERMISSIONS)
  for (const { role } of ROLE_COLUMNS) {
    for (const cell of from[role]) {
      cells.add(cell)
    }
    for (const cell of to[role]) {
      cells.add(cell)
    }
  }
  for (const permission of cells) {
    for (const { role } of ROLE_COLUMNS) {
      const granted = to[role].has(permission)
      if (from[role].has(permission) !== granted) {
        changes.push({ role, permission, granted })
      }
    }
  }
  return changes
}

export interface PermissionGraph {
  cells: PermissionCell[]
  requires: CellRequirements
  dependents: CellRequirements
  baseLocks: CellLocks
}

export const permissionGraph = (requires: CellRequirements, baseLocks: CellLocks, cells: PermissionCell[] = ALL_PERMISSIONS): PermissionGraph => {
  const dependents: CellRequirements = {}
  for (const permission of cells) {
    for (const requirement of requires[permission] ?? []) {
      const incoming = dependents[requirement] ?? []
      incoming.push(permission)
      dependents[requirement] = incoming
    }
  }

  return { cells, requires, dependents, baseLocks }
}

const changedClosure = (grants: Grants, graph: PermissionGraph, role: UserRole, permission: PermissionCell, granted: boolean): Set<PermissionCell> => {
  const changed = new Set<PermissionCell>()
  const visit = (permission: PermissionCell) => {
    if (changed.has(permission) || grants[role].has(permission) === granted) {
      return
    }

    changed.add(permission)
    const edges = granted ? graph.requires : graph.dependents
    for (const linked of edges[permission] ?? []) {
      visit(linked)
    }
  }

  visit(permission)
  return changed
}

const lockedChange = (graph: PermissionGraph, role: UserRole, changed: Set<PermissionCell>): string | undefined => {
  for (const permission of changed) {
    const reason = graph.baseLocks[role]?.[permission]
    if (reason) {
      return reason
    }
  }
}

export const projectAssignments = (grants: Grants, graph: PermissionGraph, assignments: CellAssignment[]): { grants: Grants; blocked?: string } => {
  const next = { ...grants }
  const copied = new Map<UserRole, Set<PermissionCell>>()

  for (const granted of [false, true]) {
    for (const assignment of assignments) {
      if (assignment.granted !== granted) {
        continue
      }

      const { role, permission } = assignment
      const changed = changedClosure(next, graph, role, permission, granted)
      const blocked = lockedChange(graph, role, changed)
      if (blocked) {
        return { grants, blocked }
      }
      if (changed.size === 0) {
        continue
      }

      let values = copied.get(role)
      if (!values) {
        values = new Set(grants[role])
        next[role] = values
        copied.set(role, values)
      }

      for (const cell of changed) {
        if (granted) {
          values.add(cell)
        } else {
          values.delete(cell)
        }
      }
    }
  }

  return { grants: copied.size > 0 ? next : grants }
}

export function preserveOtherGrants(grants: Grants, graph: PermissionGraph, assignments: CellAssignment[]): CellAssignment[] {
  const removals = new Map<UserRole, Set<PermissionCell>>()
  for (const { role, permission, granted } of assignments) {
    if (!granted) {
      const cells = removals.get(role) ?? new Set<PermissionCell>()
      cells.add(permission)
      removals.set(role, cells)
    }
  }

  return assignments.filter(({ role, permission, granted }) => {
    if (granted) {
      return true
    }
    // History only removes grants changed by that edit.
    for (const cell of changedClosure(grants, graph, role, permission, false)) {
      if (!removals.get(role)?.has(cell)) {
        return false
      }
    }
    return true
  })
}

export const permissionLocks = (grants: Grants, graph: PermissionGraph): CellLocks => {
  const locks: CellLocks = {}
  for (const { role } of ROLE_COLUMNS) {
    const reasons: Partial<Record<PermissionCell, string>> = {}
    for (const permission of graph.cells) {
      const changed = changedClosure(grants, graph, role, permission, !grants[role].has(permission))
      const reason = lockedChange(graph, role, changed)
      if (reason) {
        reasons[permission] = reason
      }
    }

    locks[role] = reasons
  }

  return locks
}

export function serverPermissionGraph(state: Pick<SavedRolePermissions, "requires" | "baseLocks" | "responseOptions" | "responseLocks">): PermissionGraph {
  const cells = [...ALL_PERMISSIONS, ...state.responseOptions.map(responseCell)]
  const requires: CellRequirements = { ...state.requires }
  const locks: CellLocks = {}
  for (const status of state.responseOptions) {
    requires[responseCell(status)] = ["readResponses"]
  }

  for (const { role } of ROLE_COLUMNS) {
    const roleLocks: Partial<Record<PermissionCell, string>> = { ...state.baseLocks[role] }
    for (const status of state.responseOptions) {
      const reason = state.responseLocks[role]?.[status]
      if (reason) {
        roleLocks[responseCell(status)] = reason
      }
    }
    locks[role] = roleLocks
  }
  return permissionGraph(requires, locks, cells)
}

export function saveAssignments(assignments: readonly CellAssignment[]) {
  const changes: PermissionAssignment[] = []
  const responseChanges: ResponsePermissionAssignment[] = []
  for (const assignment of assignments) {
    const status = responseStatus(assignment.permission)
    if (status === undefined) {
      changes.push({ ...assignment, permission: assignment.permission as Permission })
    } else {
      responseChanges.push({ role: assignment.role, status, granted: assignment.granted })
    }
  }
  return { changes, responseChanges }
}
