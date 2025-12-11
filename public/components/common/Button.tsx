import "./Button.scss"

import React, { useEffect, useRef, useState } from "react"
import { classSet } from "@fider/services"

interface ButtonProps {
  children?: React.ReactNode
  className?: string
  disabled?: boolean
  loading?: boolean
  href?: string
  rel?: "nofollow"
  target?: "_self" | "_blank" | "_parent" | "_top"
  type?: "button" | "submit"
  variant?: "primary" | "danger" | "secondary" | "tertiary"
  size?: "small" | "default" | "large"
  style?: React.CSSProperties
  onClick?: (event: ButtonClickEvent) => Promise<any> | void
}

export class ButtonClickEvent {
  private shouldEnable = true
  public preventEnable(): void {
    this.shouldEnable = false
  }
  public canEnable(): boolean {
    return this.shouldEnable
  }
}

export const Button: React.FC<ButtonProps> = ({ size = "default", variant = "secondary", type = "button", loading, ...props }) => {
  const [clicked, setClicked] = useState(false)
  const unmountedContainer = useRef(false)
  
  const isControlled = loading !== undefined
  const isLoading = isControlled ? loading : clicked

  useEffect(() => {
    unmountedContainer.current = false
    return () => {
      unmountedContainer.current = true
    }
  }, [])

  const className = classSet({
    "c-button": true,
    [`c-button--${size}`]: size,
    [`c-button--${variant}`]: variant,
    "c-button--loading": isLoading,
    "c-button--disabled": isLoading || props.disabled,
    [props.className || ""]: props.className,
    "shadow-sm": variant == "primary" || variant == "secondary",
  })

  let buttonContent: JSX.Element
  const onClickProp = props.onClick

  if (props.href) {
    buttonContent = (
      <a href={props.href} rel={props.rel} target={props.target} className={className} style={props.style}>
        {props.children}
      </a>
    )
  } else if (onClickProp) {
    const onClick = async (e?: React.SyntheticEvent<HTMLElement>) => {
      if (e) {
        e.preventDefault()
        e.stopPropagation()
      }

      if (isLoading) {
        return
      }

      const event = new ButtonClickEvent()
      
      if (!isControlled) {
        setClicked(true)
      }

      await onClickProp(event)

      if (!isControlled && !unmountedContainer.current && event.canEnable()) {
        setClicked(false)
      }
    }

    buttonContent = (
      <button type={type} className={className} onClick={onClick} style={props.style}>
        {props.children}
      </button>
    )
  } else {
    buttonContent = (
      <button type={type} className={className} style={props.style}>
        {props.children}
      </button>
    )
  }

  return buttonContent
}
