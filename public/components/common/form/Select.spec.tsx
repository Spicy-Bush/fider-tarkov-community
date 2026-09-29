import React, { useState } from "react"
import { fireEvent, render, screen } from "@testing-library/react"
import { expect, test } from "@jest/globals"
import { Select, SelectOption } from "./Select"

test("an empty option remains a selected value and reaches its owner", () => {
  const changed = jest.fn()
  const options: SelectOption[] = [{ value: "", label: "Default" }, { value: "Sherpa", label: "Sherpa" }]

  const Editor = () => {
    const [value, setValue] = useState("Sherpa")

    return <Select field="badge" value={value} options={options} onChange={(option) => {
      changed(option)
      setValue(option!.value)
    }} />
  }

  render(<Editor />)
  fireEvent.change(screen.getByRole("combobox"), { target: { value: "" } })

  expect(changed).toHaveBeenCalledWith(options[0])
  expect(screen.getByRole("combobox")).toHaveValue("")
})
