// import "./Dropdown.scss"

import React, { createContext, useContext, useEffect, useRef, useState } from "react"
import { classSet } from "@fider/services"

interface DropdownListItemProps {
  href?: string
  onClick?: () => void
  className?: string
  children: React.ReactNode
}

const ListItem = (props: DropdownListItemProps) => {
  const ctx = useContext(DropdownContext)
  const handleClick = () => {
    if (props.onClick) {
      props.onClick()
    }

    ctx?.close()
  }

  const baseClass = "block p-2 cursor-pointer hover:bg-surface-alt"

  if (props.href) {
    return (
      <a href={props.href} className={`${baseClass} ${props.className || ""}`}>
        {props.children}
      </a>
    )
  }

  return (
    <div onClick={handleClick} className={`${baseClass} ${props.className || ""}`}>
      {props.children}
    </div>
  )
}

const Divider = () => {
  return <hr className="m-1 bg-surface-alt h-px border-none" />
}

interface DropdownProps {
  renderHandle: JSX.Element
  position?: "left" | "right"
  onToggled?: (isOpen: boolean) => void
  children: React.ReactNode
  wide?: boolean
  fullscreenSm?: boolean
  className?: string
}

interface DropdownContextFuncs {
  close(): void
}

const DropdownContext = createContext<DropdownContextFuncs | null>(null)
DropdownContext.displayName = "DropdownContext"

export const Dropdown = (props: DropdownProps) => {
  const node = useRef<HTMLDivElement | null>(null)
  const [isOpen, setIsOpen] = useState(false)
  const position = props.position || "right"

  const changeToggleState = (newState: boolean) => {
    setIsOpen(newState)
    if (props.onToggled) {
      props.onToggled(newState)
    }
  }

  const toggleIsOpen = () => {
    changeToggleState(!isOpen)
  }

  const close = () => {
    changeToggleState(false)
  }

  const handleClick = (e: MouseEvent) => {
    if (node.current && node.current.contains(e.target as Node)) {
      return
    }

    close()
  }

  useEffect(() => {
    document.addEventListener("mousedown", handleClick)

    return () => {
      document.removeEventListener("mousedown", handleClick)
    }
  }, [])

  const listClassName = classSet({
    "absolute top-full mt-1 overflow-auto w-max min-w-40 max-w-60 py-1 bg-elevated rounded-card border border-border shadow-lg z-toolbar": true,
    "max-w-[45rem]": props.wide,
    "max-sm:fixed max-sm:left-0 max-sm:right-0 max-sm:w-auto max-sm:top-[60px] max-sm:mt-0 max-sm:max-h-[calc(100vh-60px)] max-sm:rounded-none max-sm:border-x-0": props.fullscreenSm,
    "right-0": position === "left",
  })

  const dropdownClassName = classSet({
    "inline-flex relative text-sm": true,
    [props.className || ""]: !!props.className,
  })

  return (
    <DropdownContext.Provider value={{ close }}>
      <div ref={node} className={dropdownClassName}>
        <button type="button" className="text-left bg-transparent border-none p-0 cursor-pointer inline-flex items-center" onClick={toggleIsOpen}>
          {props.renderHandle}
        </button>
        {isOpen && <div className={listClassName}>{props.children}</div>}
      </div>
    </DropdownContext.Provider>
  )
}

Dropdown.ListItem = ListItem
Dropdown.Divider = Divider
