import React from "react"
import { test, expect } from "@jest/globals"
import { render, screen } from "@testing-library/react"
import { AuthLayout } from "./AuthLayout"
import { Fider } from "@fider/services/fider"

test("first-run signup renders before a tenant exists", () => {
  Fider.initialize({ settings: {}, tenant: null })
  render(
    <AuthLayout>
      <h1>Create your community</h1>
    </AuthLayout>
  )
  expect(screen.getByRole("heading", { name: "Create your community" })).toBeTruthy()
})
