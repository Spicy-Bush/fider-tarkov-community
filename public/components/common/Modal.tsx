// import "./Modal.scss"

import React, { useEffect, useRef } from "react"
import ReactDOM from "react-dom"
import { classSet } from "@fider/services"

interface ModalWindowProps {
  children?: React.ReactNode
  className?: string
  isOpen: boolean
  size?: "small" | "large" | "fluid"
  canClose?: boolean
  center?: boolean
  manageHistory?: boolean
  onClose: () => void
}

interface ModalFooterProps {
  align?: "left" | "center" | "right"
  children?: React.ReactNode
}

const ModalWindow: React.FunctionComponent<ModalWindowProps> = ({ size = "small", canClose = true, center = true, manageHistory = true, ...props }) => {
  const root = useRef<HTMLElement>(document.getElementById("root-modal"))
  const startX = useRef<number | null>(null);
  const startY = useRef<number | null>(null);
  const pushedHistory = useRef(false);
  const onCloseRef = useRef(props.onClose);
  const canCloseRef = useRef(canClose);
  onCloseRef.current = props.onClose;
  canCloseRef.current = canClose;

  useEffect(() => {
    if (props.isOpen) {
      document.body.style.overflow = "hidden"
      document.addEventListener("keydown", keyDown, false)

      if (manageHistory && !pushedHistory.current) {
        window.history.pushState({ _modal: true }, "")
        pushedHistory.current = true
      }
    } else {
      document.body.style.overflow = ""
      document.removeEventListener("keydown", keyDown, false)
    }

    return () => {
      document.removeEventListener("keydown", keyDown, false)
    }
  }, [props.isOpen, manageHistory])

  useEffect(() => {
    if (!manageHistory) return

    const handlePopState = (event: PopStateEvent) => {
      if (pushedHistory.current && !event.state?._modal && canCloseRef.current) {
        pushedHistory.current = false
        onCloseRef.current()
      }
    }

    window.addEventListener("popstate", handlePopState)
    return () => window.removeEventListener("popstate", handlePopState)
  }, [manageHistory])

  const handleDimmerMouseDown = (e: React.MouseEvent) => {
    const modalWindow = e.currentTarget.querySelector('[data-modal-window]');
    if (modalWindow && !modalWindow.contains(e.target as Node)) {
      startX.current = e.clientX;
      startY.current = e.clientY;
    } else {
      startX.current = null;
      startY.current = null;
    }
  };

  const handleDimmerMouseUp = (e: React.MouseEvent) => {
    const modalWindow = e.currentTarget.querySelector('[data-modal-window]');
    if (
      modalWindow && 
      !modalWindow.contains(e.target as Node) &&
      startX.current !== null && 
      startY.current !== null &&
      Math.abs(e.clientX - startX.current) < 5 &&
      Math.abs(e.clientY - startY.current) < 5 &&
      canClose
    ) {
      close();
    }
    
    startX.current = null;
    startY.current = null;
  };

  const keyDown = (event: KeyboardEvent) => {
    if (event.keyCode === 27) {
      close()
    }
  }

  const close = () => {
    if (!canCloseRef.current) return
    
    if (pushedHistory.current) {
      window.history.back()
    } else {
      onCloseRef.current()
    }
  }

  if (!props.isOpen || !root.current) {
    return null
  }

  const sizeClasses = {
    small: "w-full md:w-[500px]",
    large: "w-full md:w-[750px]",
    fluid: "w-full",
  }

  const className = classSet({
    "z-401 text-left bg-overlay border-none rounded-modal animate-[windowFadeIn_0.5s] max-sm:w-full max-sm:h-full max-sm:rounded-none max-sm:flex max-sm:flex-col": true,
    [`${props.className}`]: !!props.className,
    "text-center": center,
    [sizeClasses[size]]: true,
  })

  return ReactDOM.createPortal(
    <div 
      aria-disabled={true} 
      className="fixed inset-0 w-full h-full text-center align-middle p-4 z-overlay flex justify-center items-start overflow-y-auto animate-[dimmerFadeIn_0.5s] bg-black/80 max-sm:p-0 max-sm:items-stretch"
      onMouseDown={handleDimmerMouseDown}
      onMouseUp={handleDimmerMouseUp}
    >
      <div className="min-h-[calc(100vh-2rem)] flex items-center max-sm:min-h-full max-sm:w-full">
        <div 
          className={className} 
          data-testid="modal"
          data-modal-window
        >
          {props.children}
        </div>
      </div>
    </div>,
    root.current
  )
}

const Header = (props: { children: React.ReactNode }) => (
  <div className="text-lg font-semibold px-5 py-4 border-b border-border">{props.children}</div>
)

const Content = (props: { children: React.ReactNode }) => (
  <div className="px-5 py-4 max-sm:flex-1 max-sm:overflow-y-auto">{props.children}</div>
)

const Footer = (props: ModalFooterProps) => {
  const align = props.align || "right"
  const alignClass = {
    left: "text-left",
    center: "text-center",
    right: "text-right",
  }
  return (
    <div className={`bg-surface p-2.5 rounded-b border-t border-border-strong ${alignClass[align]}`}>
      {props.children}
    </div>
  )
}

export const Modal = {
  Window: ModalWindow,
  Header,
  Content,
  Footer,
}
