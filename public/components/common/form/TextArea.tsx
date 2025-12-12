import React from "react"
import { classSet } from "@fider/services"
import { ValidationContext } from "../"
import { DisplayError, hasError } from "./DisplayError"

// import "./TextArea.scss"

interface TextAreaProps {
  children?: React.ReactNode
  label?: string
  field: string
  value?: string
  disabled?: boolean
  minRows?: number
  placeholder?: string
  afterLabel?: JSX.Element
  onChange?: (value: string, selectionStart?: number) => void
  onKeyDown?: (e: React.KeyboardEvent<HTMLTextAreaElement>) => void
  inputRef?: React.MutableRefObject<any>
  onFocus?: React.FocusEventHandler<HTMLTextAreaElement>
  className?: string
  maxLength?: number
}

export const TextArea: React.FunctionComponent<TextAreaProps> = (props) => {
  const onChange = (e: React.FormEvent<HTMLTextAreaElement>) => {
    if (props.onChange) {
      props.onChange(e.currentTarget.value, e.currentTarget.selectionStart)
    }
  }

  const onKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (props.onKeyDown) {
      props.onKeyDown(e)
    }
  }

  const minRows = props.minRows || 3

  return (
    <ValidationContext.Consumer>
      {(ctx) => (
        <>
          <div className="mb-4">
            {!!props.label && (
              <label htmlFor={`input-${props.field}`} className="block text-sm font-medium text-foreground mb-1">
                {props.label}
                {props.afterLabel}
              </label>
            )}
            <div 
              className="grid grid-cols-1 after:content-[attr(data-value)_'_'] after:invisible after:whitespace-pre-wrap after:break-all after:col-start-1 after:row-start-1 after:font-inherit after:leading-relaxed after:p-2 after:border after:border-border after:rounded-input"
              data-value={props.value || ""}
            >
              <textarea
                className={classSet({
                  "col-start-1 row-start-1 font-inherit leading-relaxed p-2 border border-border rounded-input whitespace-pre-wrap break-all bg-elevated resize-none overflow-hidden text-foreground": true,
                  "border-danger": hasError(props.field, ctx.error),
                  "opacity-50 cursor-not-allowed": props.disabled,
                  [props.className || ""]: props.className,
                })}
                id={`input-${props.field}`}
                disabled={props.disabled}
                onChange={onChange}
                onKeyDown={onKeyDown}
                value={props.value}
                rows={minRows}
                placeholder={props.placeholder}
                ref={props.inputRef}
                onFocus={props.onFocus}
                maxLength={props.maxLength}
              />
            </div>
            <DisplayError fields={[props.field]} error={ctx.error} />
            {props.children}
          </div>
        </>
      )}
    </ValidationContext.Consumer>
  )
}
