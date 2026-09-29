import { expect, test } from "@jest/globals"
import { PermissionAssignment, UserRole } from "@fider/models"
import { PERMISSIONS, PERMISSION_GROUPS } from "./catalog"
import { diffGrants, permissionGraph, permissionLocks, projectAssignments, showChoices, toGrants, toLists } from "./grants"

const emptyGrants = () => toGrants({ visitor: [], helper: [], moderator: [], collaborator: [], administrator: [] })
const graph = permissionGraph({ changeUserRoles: ["manageMembers"], manageMembers: ["readProfiles"] }, {})

const grantRoles: PermissionAssignment = { role: UserRole.Visitor, permission: "changeUserRoles", granted: true }
const revokeProfiles: PermissionAssignment = { role: UserRole.Visitor, permission: "readProfiles", granted: false }

test("every permission is listed in exactly one group", () => {
  const listed = PERMISSION_GROUPS.flatMap((group) => group.permissions).sort()
  expect(listed).toEqual(Object.keys(PERMISSIONS).sort())
})

test("pending save reconstruction preserves its exact submitted values", () => {
  const grants = emptyGrants()
  const choices = showChoices(grants, [grantRoles])

  expect(toLists(choices).visitor).toEqual(["changeUserRoles"])
  expect(toLists(grants).visitor).toEqual([])
  expect(diffGrants(grants, choices)).toEqual([grantRoles])
})

test("grants include transitive requirements and revocation removes dependents", () => {
  const original = emptyGrants()
  const granted = projectAssignments(original, graph, [grantRoles])
  expect([...granted.grants.visitor].sort()).toEqual(["changeUserRoles", "manageMembers", "readProfiles"])
  expect([...original.visitor]).toEqual([])

  const revoked = projectAssignments(granted.grants, graph, [revokeProfiles])
  expect([...revoked.grants.visitor]).toEqual([])
  expect(projectAssignments(revoked.grants, graph, [revokeProfiles]).grants).toBe(revoked.grants)
})

test("separate clicks preserve order while a batch applies revocations before grants", () => {
  const original = emptyGrants()
  const granted = projectAssignments(original, graph, [grantRoles])
  const clicked = projectAssignments(granted.grants, graph, [revokeProfiles])
  const batched = projectAssignments(original, graph, [grantRoles, revokeProfiles])

  expect([...clicked.grants.visitor]).toEqual([])
  expect([...batched.grants.visitor].sort()).toEqual(["changeUserRoles", "manageMembers", "readProfiles"])
})

test("a locked requirement rejects the entire batch without mutating input", () => {
  const original = emptyGrants()
  const locked = permissionGraph(graph.requires, { visitor: { readProfiles: "Requires a higher role" } })
  const projected = projectAssignments(original, locked, [
    { role: UserRole.Helper, permission: "lockPosts", granted: true },
    grantRoles,
  ])

  expect(projected).toEqual({ grants: original, blocked: "Requires a higher role" })
  expect([...original.helper]).toEqual([])
  expect(permissionLocks(original, locked).visitor?.changeUserRoles).toBe("Requires a higher role")
})

test("a locked granted dependent prevents removing its requirement", () => {
  const granted = projectAssignments(emptyGrants(), graph, [grantRoles]).grants
  const locked = permissionGraph(graph.requires, { visitor: { changeUserRoles: "Requires a higher role" } })

  expect(permissionLocks(granted, locked).visitor?.readProfiles).toBe("Requires a higher role")
  expect(projectAssignments(granted, locked, [revokeProfiles]).grants).toBe(granted)
})
