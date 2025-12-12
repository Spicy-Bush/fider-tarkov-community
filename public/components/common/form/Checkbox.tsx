// Checkbox converted to Tailwind

import React, { useState, useEffect } from "react"
import { classSet } from "@fider/services"
import { DisplayError, ValidationContext, hasError } from "../"
import { HStack } from "@fider/components/layout"

interface CheckboxProps {
  children?: React.ReactNode
  field: string
  checked?: boolean
  onChange?: (checked: boolean) => void
  expandedHitbox?: boolean
}

export const Checkbox: React.FC<CheckboxProps> = (props) => {
  const [checked, setChecked] = useState<boolean>(props.checked || false)

  useEffect(() => {
    setChecked(props.checked || false)
  }, [props.checked])

  const onChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const isChecked: boolean = e.currentTarget.checked

    setChecked(isChecked)
    if (props.onChange) {
      props.onChange(isChecked)
    }
  }

  const handleHitboxClick = () => {
    const newChecked = !checked
    setChecked(newChecked)
    if (props.onChange) {
      props.onChange(newChecked)
    }
  }

  return (
    <ValidationContext.Consumer>
      {(ctx) => (
        <div
          className={classSet({
            "": true,
            "has-error": hasError(props.field, ctx.error),
          })}
        >
          <div className="relative">
            {props.expandedHitbox && (
              <div 
                className="absolute -inset-3 cursor-pointer z-1" 
                onClick={handleHitboxClick} 
              />
            )}
            <HStack>
              <input 
                id={`input-${props.field}`} 
                type="checkbox" 
                checked={checked} 
                onChange={onChange}
                className={classSet({
                  "m-0 h-4 w-4 appearance-none border border-border rounded-badge bg-elevated cursor-pointer": true,
                  "checked:border-transparent checked:bg-primary checked:bg-size-[100%_100%] checked:bg-center checked:bg-no-repeat": true,
                  "relative z-2 pointer-events-none": props.expandedHitbox,
                })}
                style={{
                  backgroundImage: checked ? "url(\"data:image/svg+xml,%3csvg viewBox='0 0 16 16' fill='white' xmlns='http://www.w3.org/2000/svg'%3e%3cpath d='M12.207 4.793a1 1 0 010 1.414l-5 5a1 1 0 01-1.414 0l-2-2a1 1 0 011.414-1.414L6.5 9.086l4.293-4.293a1 1 0 011.414 0z'/%3e%3c/svg%3e\")" : "none"
                }}
              />
              <label htmlFor={`input-${props.field}`} className="text-sm cursor-pointer">
                {props.children}
              </label>
            </HStack>
            <DisplayError fields={[props.field]} error={ctx.error} />
          </div>
        </div>
      )}
    </ValidationContext.Consumer>
  )
}
