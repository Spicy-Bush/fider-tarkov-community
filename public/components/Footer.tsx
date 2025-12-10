import "./Footer.scss"
import React, { useEffect, useRef, useState } from "react"
import { Icon } from "@fider/components/common"
import { heroiconsChevronUp, heroiconsChevronDown } from "@fider/icons.generated"

type FooterState = "revealing" | "visible" | "freezing" | "hidden" | "unhiding"

const Footer: React.FC = () => {
  const currentYear = new Date().getFullYear()
  const ref = useRef<HTMLElement>(null)
  const state = useRef<FooterState>("revealing")
  const lastScrollY = useRef(0)
  const anchorY = useRef(0)
  const ticking = useRef(false)
  const [expanded, setExpanded] = useState(false)

  useEffect(() => {
    const clamp = (val: number) => Math.min(1, Math.max(0, val))

    const getScrollY = () => Math.max(
      window.scrollY || 0,
      window.pageYOffset || 0,
      document.documentElement.scrollTop || 0,
      document.body.scrollTop || 0
    )

    const updateFooter = () => {
      const footer = ref.current
      if (!footer) return

      const scrollY = getScrollY()
      const delta = scrollY - lastScrollY.current
      const footerHeight = footer.offsetHeight || 100
      const viewportHeight = window.innerHeight

      const calcProgress = (): number => {
        switch (state.current) {
          case "revealing": return clamp(scrollY / viewportHeight)
          case "freezing": return clamp((anchorY.current - scrollY) / footerHeight)
          case "unhiding": return clamp((scrollY - anchorY.current) / footerHeight)
          default: return 0
        }
      }

      const progress = calcProgress()
      let translateY = 0

      switch (state.current) {
        case "revealing":
          translateY = (1 - progress) * 100
          if (progress >= 1) state.current = "visible"
          break
        case "visible":
          translateY = 0
          if (delta < 0) { state.current = "freezing"; anchorY.current = scrollY }
          break
        case "freezing":
          translateY = progress * 100
          if (progress >= 1) state.current = "hidden"
          else if (delta > 0 && progress <= 0) state.current = "visible"
          break
        case "hidden":
          translateY = 100
          if (delta > 0) { state.current = "unhiding"; anchorY.current = scrollY }
          break
        case "unhiding":
          translateY = (1 - progress) * 100
          if (progress >= 1) state.current = "visible"
          else if (delta < 0 && progress <= 0) state.current = "hidden"
          break
      }

      footer.style.transform = `translateY(${translateY}%)`
      lastScrollY.current = scrollY
      ticking.current = false
    }

    const onScroll = () => {
      if (!ticking.current) {
        requestAnimationFrame(updateFooter)
        ticking.current = true
      }
    }

    window.addEventListener("scroll", onScroll, { passive: true })
    document.body.addEventListener("scroll", onScroll, { passive: true })
    updateFooter()
    return () => {
      window.removeEventListener("scroll", onScroll)
      document.body.removeEventListener("scroll", onScroll)
    }
  }, [])

  return (
    <footer ref={ref} className={`site-footer ${expanded ? "site-footer--expanded" : ""}`} style={{ transform: "translateY(100%)" }}>
      <button 
        className="site-footer__bar"
        onClick={() => setExpanded(!expanded)}
        aria-expanded={expanded}
        aria-label={expanded ? "Collapse footer links" : "Expand footer links"}
      >
        <span>© {currentYear} Spicy Bush Team</span>
        <Icon className="site-footer__toggle" sprite={expanded ? heroiconsChevronDown : heroiconsChevronUp} height="20" width="20" />
      </button>
      <nav className="site-footer__nav">
        <ul>
          <li>
            <a href="/terms">
              <span className="site-footer__text--short">Terms</span>
              <span className="site-footer__text--full">Terms & Conditions</span>
            </a>
          </li>
          <li>
            <a href="/privacy">
              <span className="site-footer__text--short">Privacy</span>
              <span className="site-footer__text--full">Privacy Policy</span>
            </a>
          </li>
          <li>
            <a href="mailto:contact@tarkov.community">Contact</a>
          </li>
          <li>
            <a href="https://discord.gg/escapefromtarkovofficial" target="_blank" rel="noreferrer">
              <span className="site-footer__text--short">Discord</span>
              <span className="site-footer__text--full">Official Tarkov Discord</span>
            </a>
          </li>
          <li>
            <a href="https://www.escapefromtarkov.com/support" target="_blank" rel="noreferrer">
              <span className="site-footer__text--short">Support</span>
              <span className="site-footer__text--full">Official Tarkov Support</span>
            </a>
          </li>
          <li>
            <a href="https://ko-fi.com/tarkovcommunity" target="_blank" rel="noreferrer">
              <span className="site-footer__text--short">Ko-fi</span>
              <span className="site-footer__text--full">Support us on Ko-fi</span>
            </a>
          </li>
        </ul>
      </nav>
      <p className="site-footer__copyright">© {currentYear} - This website is proudly made by the Spicy Bush Team.</p>
    </footer>
  )
}

export default Footer
