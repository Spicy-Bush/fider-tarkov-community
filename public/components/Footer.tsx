import "./Footer.scss"
import React, { useState, useEffect, useRef } from "react"

type FooterState = "revealing" | "visible" | "freezing" | "hidden" | "unhiding"

const Footer: React.FC = () => {
  const currentYear = new Date().getFullYear()
  const [translateY, setTranslateY] = useState(100)
  const ref = useRef<HTMLElement>(null)
  const state = useRef<FooterState>("revealing")
  const lastScrollY = useRef(0)
  const anchorY = useRef(0)

  useEffect(() => {
    const clamp = (val: number) => Math.min(1, Math.max(0, val))

    const onScroll = () => {
      const scrollY = window.scrollY
      const delta = scrollY - lastScrollY.current
      const footerHeight = ref.current?.offsetHeight || 100
      const docHeight = document.documentElement.scrollHeight - window.innerHeight

      const calcProgress = (): number => {
        switch (state.current) {
          case "revealing": return clamp(docHeight > 0 ? (scrollY / docHeight) / 0.25 : 0)
          case "freezing": return clamp((anchorY.current - scrollY) / footerHeight)
          case "unhiding": return clamp((scrollY - anchorY.current) / footerHeight)
          default: return 0
        }
      }

      const progress = calcProgress()

      switch (state.current) {
        case "revealing":
          setTranslateY((1 - progress) * 100)
          if (progress >= 1) state.current = "visible"
          break
        case "visible":
          setTranslateY(0)
          if (delta < 0) { state.current = "freezing"; anchorY.current = scrollY }
          break
        case "freezing":
          setTranslateY(progress * 100)
          if (progress >= 1) state.current = "hidden"
          else if (delta > 0 && progress <= 0) state.current = "visible"
          break
        case "hidden":
          setTranslateY(100)
          if (delta > 0) { state.current = "unhiding"; anchorY.current = scrollY }
          break
        case "unhiding":
          setTranslateY((1 - progress) * 100)
          if (progress >= 1) state.current = "visible"
          else if (delta < 0 && progress <= 0) state.current = "hidden"
          break
      }

      lastScrollY.current = scrollY
    }

    window.addEventListener("scroll", onScroll, { passive: true })
    onScroll()
    return () => window.removeEventListener("scroll", onScroll)
  }, [])

  return (
    <footer ref={ref} className="site-footer" style={{ transform: `translateY(${translateY}%)` }}>
      <nav>
        <ul>
          <li>
            <a href="/terms">Terms & Conditions</a>
          </li>
          <li>
            <a href="/privacy">Privacy Policy</a>
          </li>
          <li>
            <a href="mailto:contact@tarkov.community">Contact</a>
          </li>
          <li>
            <a href="https://discord.gg/escapefromtarkovofficial" target="_blank" rel="noreferrer">
              Official Tarkov Discord
            </a>
          </li>
          <li>
            <a href="https://www.escapefromtarkov.com/support" target="_blank" rel="noreferrer">
              Official Tarkov Support
            </a>
          </li>
        </ul>
      </nav>
      <p>© {currentYear} - This website is proudly made by the Spicy Bush Team.</p>
    </footer>
  )
}

export default Footer
