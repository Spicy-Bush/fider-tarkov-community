// Select converted to Tailwind

import React from "react"
import { classSet } from "@fider/services"
import { ValidationContext } from "./Form"
import { DisplayError, hasError } from "./DisplayError"

export interface SelectOption {
  value: string
  label: string
}

interface SelectProps {
  children?: React.ReactNode
  field: string
  label?: string
  maxLength?: number
  defaultValue?: string
  value?: string
  options: SelectOption[]
  onChange?: (option?: SelectOption) => void
  disabled?: boolean
}

export const Select: React.FunctionComponent<SelectProps> = (props) => {
  const Options = Array.isArray(props.options) ? props.options : [];
  const isControlled = props.value !== undefined
  
  const getOption = (value?: string) => {
    if (value && Options.length > 0) {
      const filtered = Options.filter((x) => x.value === value)
      if (filtered && filtered.length > 0) {
        return filtered[0]
      }
    }
    return undefined;
  }
  
  const [internalSelected, setInternalSelected] = React.useState<SelectOption | undefined>(getOption(props.defaultValue))
  const selected = isControlled ? getOption(props.value) : internalSelected
  
  const onChange = (e: React.FormEvent<HTMLSelectElement>) => {
    let newSelected: SelectOption | undefined
    if (e.currentTarget.value) {
      const options = Options.filter((o) => o.value === e.currentTarget.value)
      if (options && options.length > 0) {
        newSelected = options[0]
      }
    }

    if (!isControlled) {
      setInternalSelected(newSelected)
    }
    if (props.onChange) {
      props.onChange(newSelected)
    }
  }

  return (
    <ValidationContext.Consumer>
      {(ctx) => (
        <>
          <div className="mb-4">
            {!!props.label && <label htmlFor={`input-${props.field}`} className="block text-sm font-medium mb-1">{props.label}</label>}
            <select
              className={classSet({
                "w-full bg-elevated p-2 border rounded-input appearance-none bg-no-repeat pr-10 disabled:opacity-45 disabled:cursor-not-allowed disabled:pointer-events-none": true,
                "border-border": !hasError(props.field, ctx.error),
                "border-danger": hasError(props.field, ctx.error),
              })}
              style={{
                backgroundImage: "url(\"data:image/svg+xml,%3csvg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 20 20'%3e%3cpath stroke='%236b7280' stroke-linecap='round' stroke-linejoin='round' stroke-width='1.5' d='M6 8l4 4 4-4'/%3e%3c/svg%3e\")",
                backgroundPosition: "right 0.5rem center",
                backgroundSize: "1.5em 1.5em",
              }}
              value={selected?.value || ""}
              id={`input-${props.field}`}
              onChange={onChange}
              disabled={props.disabled}
            >
              {Options.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
            <DisplayError fields={[props.field]} error={ctx.error} />
            {props.children}
          </div>
        </>
      )}
    </ValidationContext.Consumer>
  )
}
