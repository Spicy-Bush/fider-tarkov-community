// import "./Form.scss"

import React from "react"
import { Failure } from "@fider/services"
import { classSet } from "@fider/services/utils"
import { DisplayError } from "@fider/components/common/form/DisplayError"

interface ValidationContext {
  error?: Failure
}

interface FormProps {
  children?: React.ReactNode
  className?: string
  error?: Failure
}

export const ValidationContext = React.createContext<ValidationContext>({})

export const Form: React.FunctionComponent<FormProps> = (props) => {
  const className = classSet({
    "[&>*:last-child]:mb-0": true,
    [props.className || ""]: props.className,
  })

  return (
    <form autoComplete="off" className={className}>
      <DisplayError error={props.error} />
      <ValidationContext.Provider value={{ error: props.error }}>{props.children}</ValidationContext.Provider>
    </form>
  )
}
