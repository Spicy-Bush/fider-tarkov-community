// import "./Button.scss"

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

const sizeClasses = {
  small: "px-2.5 py-1.5 text-xs [&_svg]:h-3 [&_img]:h-3",
  default: "px-3 py-2 text-sm [&_svg]:h-4 [&_img]:h-4",
  large: "px-3.5 py-2.5 text-lg [&_svg]:h-5 [&_img]:h-5",
}

const variantClasses = {
  primary: "text-white bg-primary border border-primary hover:bg-primary-hover shadow-sm",
  secondary: "text-foreground bg-secondary border border-border hover:bg-secondary-hover shadow-sm",
  tertiary: "text-foreground bg-transparent border-none hover:underline",
  danger: "text-danger bg-elevated border border-border hover:text-white hover:border-danger hover:bg-danger",
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
    "inline-flex items-center cursor-pointer rounded-button font-medium whitespace-nowrap leading-tight active:scale-[0.99] [&_svg+*]:ml-1 [&_img+*]:ml-1": true,
    [sizeClasses[size]]: true,
    [variantClasses[variant]]: true,
    "relative cursor-default text-transparent opacity-100 pointer-events-auto before:absolute before:content-[''] before:top-1/2 before:left-1/2 before:-mt-2.5 before:-ml-2.5 before:w-5 before:h-5 before:rounded-full before:border-2 before:border-muted after:absolute after:content-[''] after:top-1/2 after:left-1/2 after:-mt-2.5 after:-ml-2.5 after:w-5 after:h-5 after:animate-spin after:rounded-full after:border-2 after:border-t-white after:border-r-transparent after:border-b-transparent after:border-l-transparent": isLoading,
    "opacity-50 cursor-not-allowed pointer-events-none": isLoading || props.disabled,
    [props.className || ""]: props.className,
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
