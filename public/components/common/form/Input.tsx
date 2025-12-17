import React from "react"
import { classSet } from "@fider/services"
import { ValidationContext } from "./Form"
import { DisplayError, hasError } from "./DisplayError"
import { Icon } from "@fider/components"

// import "./Input.scss"
import { HStack } from "@fider/components/layout"

interface InputProps {
  children?: React.ReactNode
  field: string
  label?: string
  className?: string
  autoComplete?: string
  autoFocus?: boolean
  noTabFocus?: boolean
  noMargin?: boolean
  afterLabel?: JSX.Element
  icon?: SpriteSymbol
  maxLength?: number
  value?: string
  disabled?: boolean
  suffix?: string | JSX.Element
  placeholder?: string
  type?: string
  min?: number
  max?: number
  onIconClick?: () => void
  onFocus?: () => void
  inputRef?: React.MutableRefObject<any>
  onChange?: (value: string) => void
}

export const Input: React.FunctionComponent<InputProps> = (props) => {
  const onChange = (e: React.FormEvent<HTMLInputElement>) => {
    if (props.onChange) {
      props.onChange(e.currentTarget.value)
    }
  }

  const suffix = typeof props.suffix === "string" ? <span className="flex items-center bg-surface-alt p-2 rounded-r-md">{props.suffix}</span> : props.suffix

  const icon = props.icon ? <Icon sprite={props.icon} onClick={props.onIconClick} className={classSet({ "cursor-pointer": !!props.onIconClick })} /> : undefined

  return (
    <ValidationContext.Consumer>
      {(ctx) => (
        <div className={classSet({ "mb-4": !props.noMargin, [`${props.className}`]: props.className })}>
          {!!props.label && (
            <label htmlFor={`input-${props.field}`} className="block text-sm font-medium text-foreground mb-1">
              {props.label}
              {props.afterLabel}
            </label>
          )}
          <HStack spacing={0} align={props.icon ? "center" : "start"} className="relative">
            <input
              className={classSet({
                "w-full px-3 py-2 text-base bg-elevated border border-border rounded-input appearance-none leading-relaxed text-foreground": true,
                "pr-8": !!props.icon,
                "border-danger": hasError(props.field, ctx.error),
                "rounded-r-none": !!suffix,
                "opacity-50 cursor-not-allowed": props.disabled,
              })}
              id={`input-${props.field}`}
              type="text"
              autoComplete={props.autoComplete}
              tabIndex={props.noTabFocus ? -1 : undefined}
              ref={props.inputRef}
              autoFocus={props.autoFocus}
              onFocus={props.onFocus}
              maxLength={props.maxLength}
              disabled={props.disabled}
              value={props.value}
              placeholder={props.placeholder}
              onChange={onChange}
            />
            {icon && <div className="absolute right-0 w-9 p-2">{icon}</div>}
            {suffix}
          </HStack>
          <DisplayError fields={[props.field]} error={ctx.error} />
          {props.children}
        </div>
      )}
    </ValidationContext.Consumer>
  )
}
