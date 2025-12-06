import "./Footer.scss"
import React, { useState, useEffect, useRef } from "react"

const Footer: React.FC = () => {
  const currentYear = new Date().getFullYear()
  const [revealProgress, setRevealProgress] = useState(0)
  const footerRef = useRef<HTMLElement>(null)

  useEffect(() => {
    const handleScroll = () => {
      const scrollTop = window.scrollY
      const docHeight = document.documentElement.scrollHeight - window.innerHeight
      const scrollPercent = docHeight > 0 ? (scrollTop / docHeight) * 100 : 0
      
      const startReveal = 0
      const endReveal = 25
      
      if (scrollPercent <= startReveal) {
        setRevealProgress(0)
      } else if (scrollPercent >= endReveal) {
        setRevealProgress(1)
      } else {
        const progress = (scrollPercent - startReveal) / (endReveal - startReveal)
        setRevealProgress(progress)
      }
    }

    window.addEventListener("scroll", handleScroll, { passive: true })
    handleScroll()

    return () => window.removeEventListener("scroll", handleScroll)
  }, [])

  const translateY = (1 - revealProgress) * 100

  return (
    <footer 
      ref={footerRef}
      className="site-footer"
      style={{ transform: `translateY(${translateY}%)` }}
    >
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
