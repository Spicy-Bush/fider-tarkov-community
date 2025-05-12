import React from "react"
import { classSet } from "@fider/services"
import { ValidationContext } from "./Form"
import { DisplayError, hasError } from "./DisplayError"

import "./Select.scss"

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
  options: SelectOption[]
  onChange?: (option?: SelectOption) => void
  disabled?: boolean
}

export const Select: React.FunctionComponent<SelectProps> = (props) => {
  const Options = Array.isArray(props.options) ? props.options : [];
  
  const getOption = (value?: string) => {
    if (value && Options.length > 0) {
      const filtered = Options.filter((x) => x.value === value)
      if (filtered && filtered.length > 0) {
        return filtered[0]
      }
    }
    return undefined;
  }
  
  const [selected, setSelected] = React.useState<SelectOption | undefined>(getOption(props.defaultValue))
  
  const onChange = (e: React.FormEvent<HTMLSelectElement>) => {
    let selected: SelectOption | undefined
    if (e.currentTarget.value) {
      const options = Options.filter((o) => o.value === e.currentTarget.value)
      if (options && options.length > 0) {
        selected = options[0]
      }
    }

    setSelected(selected)
    if (props.onChange) {
      props.onChange(selected)
    }
  }

  return (
    <ValidationContext.Consumer>
      {(ctx) => (
        <>
          <div className="c-form-field">
            {!!props.label && <label htmlFor={`input-${props.field}`}>{props.label}</label>}
            <select
              className={classSet({
                "c-select": true,
                "c-select--error": hasError(props.field, ctx.error),
              })}
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
