import React from "react"
import { render, screen } from "@testing-library/react"
import { beforeEach, expect, test } from "@jest/globals"
import { UserRole, VisualRole } from "@fider/models"
import { UserName } from "./UserName"
import { Avatar } from "./Avatar"
import { Fider } from "@fider/services/fider"

beforeEach(() => {
  Fider.initialize({ settings: {}, user: { id: 7 } })
})

test.each([7, 8])("name and avatar links resolve the profile of user %i", (id) => {
  const user = { id, name: "Member", permissions: { readProfile: true } }
  const { container } = render(<><UserName user={user} /><Avatar user={user} /></>)
  const links = container.querySelectorAll("a")

  expect(links).toHaveLength(2)
  for (const link of links) {
    expect(link).toHaveAttribute("href", id === 7 ? "/profile" : "/profile/8")
  }
})

test("the displayed badge follows the server value independently of permission role", () => {
  const user = {
    id: 7,
    name: "Member",
    role: UserRole.Administrator,
    visualRole: VisualRole.Visitor,
    permissions: { readProfile: true },
  }
  const view = render(<UserName user={user} />)

  expect(screen.getByRole("link", { name: "Member" })).toHaveClass("vr-Visitor")
  expect(document.querySelector(".c-username--visualrole")).toBeNull()

  view.rerender(<UserName user={{ ...user, visualRole: VisualRole.Sherpa }} />)

  expect(screen.getByRole("link", { name: "Member" })).toHaveClass("vr-Sherpa")
  expect(document.querySelector(".c-username--visualrole")).not.toBeNull()
})
