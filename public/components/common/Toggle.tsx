// Toggle converted to Tailwind

import React, { useState, useEffect } from "react"
import { classSet } from "@fider/services"
import { HStack } from "../layout"
import { DisplayError, ValidationContext } from "@fider/components"

interface ToggleProps {
  field?: string
  label?: string
  active: boolean
  disabled?: boolean
  onToggle?: (active: boolean) => void
}

export const Toggle: React.FC<ToggleProps> = (props) => {
  const [active, setActive] = useState(props.active)

  useEffect(() => {
    setActive(props.active)
  }, [props.active])

  const toggle = () => {
    if (props.disabled) {
      return
    }

    const newActive = !active
    setActive(newActive)
    if (props.onToggle) {
      props.onToggle(newActive)
    }
  }

  const buttonClasses = classSet({
    "relative inline-flex h-4 w-8 border-2 border-transparent rounded-full cursor-pointer transition-colors duration-75 ease-in-out": true,
    "bg-surface-alt": !active,
    "bg-success": active,
    "opacity-45 cursor-not-allowed pointer-events-none": !!props.disabled,
  })

  const knobClasses = classSet({
    "pointer-events-none inline-block h-3 w-3 rounded-full bg-elevated transition-transform duration-75 ease-in-out": true,
    "translate-x-0": !active,
    "translate-x-4": active,
  })

  return (
    <ValidationContext.Consumer>
      {(ctx) => (
        <>
          <HStack spacing={2}>
            <button onClick={toggle} type="button" className={buttonClasses} role="switch" aria-checked={active}>
              <span aria-hidden="true" className={knobClasses}></span>
            </button>
            {props.label && <span className="text-sm">{props.label}</span>}
          </HStack>
          {props.field && <DisplayError fields={[props.field]} error={ctx.error} />}
        </>
      )}
    </ValidationContext.Consumer>
  )
}
