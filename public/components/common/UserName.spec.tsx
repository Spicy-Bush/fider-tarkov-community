import React from "react"
import { render, screen } from "@testing-library/react"
import { expect, test } from "@jest/globals"
import { UserRole, VisualRole } from "@fider/models"
import { UserName } from "./UserName"

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
