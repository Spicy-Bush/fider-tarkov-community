// import "./Loader.scss"

import React, { useState } from "react"
import { useTimeout } from "@fider/hooks"
import { classSet } from "@fider/services"

interface LoaderProps {
  text?: string
  className?: string
}

export function Loader(props: LoaderProps) {
  const [show, setShow] = useState(false)

  useTimeout(() => {
    setShow(true)
  }, 500)

  const className = classSet({
    "": true,
    [props.className || ""]: props.className,
  })

  return show ? (
    <div className={className}>
      <div className="block relative text-center z-1000 w-[30px] h-[30px] mx-auto my-2.5 before:absolute before:content-[''] before:top-0 before:left-1/2 before:w-full before:h-full before:rounded-full before:border-[0.2em] before:border-border before:-ml-[15px] after:absolute after:content-[''] after:top-0 after:left-1/2 after:animate-spin after:rounded-full after:border-[0.2em] after:border-t-muted after:border-r-transparent after:border-b-transparent after:border-l-transparent after:w-[30px] after:h-[30px] after:-ml-[15px]" />
      {props.text && <span className="block text-center text-xs text-muted">{props.text}</span>}
    </div>
  ) : null
}
