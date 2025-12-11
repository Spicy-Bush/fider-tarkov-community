import "./Modal.scss"

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
  // use refs for onClose and canClose so the popstate handler always has current values.
  // without refs, the handler would capture stale values from when it was created,
  // and react's effect cleanup/setup cycle during re renders could cause the handler
  // to miss events or use outdated callbacks
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
    const modalWindow = e.currentTarget.querySelector('.c-modal-window');
    if (modalWindow && !modalWindow.contains(e.target as Node)) {
      startX.current = e.clientX;
      startY.current = e.clientY;
    } else {
      startX.current = null;
      startY.current = null;
    }
  };

  const handleDimmerMouseUp = (e: React.MouseEvent) => {
    const modalWindow = e.currentTarget.querySelector('.c-modal-window');
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

  const className = classSet({
    "c-modal-window": true,
    [`${props.className}`]: !!props.className,
    "c-modal-window--center": center,
    [`c-modal-window--${size}`]: true,
  })

  return ReactDOM.createPortal(
    <div 
      aria-disabled={true} 
      className="c-modal-dimmer" 
      onMouseDown={handleDimmerMouseDown}
      onMouseUp={handleDimmerMouseUp}
    >
      <div className="c-modal-scroller">
        <div 
          className={className} 
          data-testid="modal"
        >
          {props.children}
        </div>
      </div>
    </div>,
    root.current
  )
}
const Header = (props: { children: React.ReactNode }) => <div className="c-modal-header">{props.children}</div>
const Content = (props: { children: React.ReactNode }) => <div className="c-modal-content">{props.children}</div>
const Footer = (props: ModalFooterProps) => {
  const align = props.align || "right"
  const className = classSet({
    "c-modal-footer": true,
    [`c-modal-footer--${align}`]: true,
  })
  return <div className={className}>{props.children}</div>
}

export const Modal = {
  Window: ModalWindow,
  Header,
  Content,
  Footer,
}
