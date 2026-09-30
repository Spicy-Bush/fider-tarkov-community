import React, { useEffect, useState } from "react"
import { Button } from "@fider/components/common/Button"
import { Modal } from "@fider/components/common/Modal"
import { cookieConsent, CookieChoices, useCookieConsent } from "@fider/services/cookieConsent"
import { startGoogle } from "@fider/services/google"
import { useFider } from "@fider/hooks/use-fider"

const declined: CookieChoices = { analytics: false }

export function CookiePreferences() {
  const fider = useFider()
  const choices = useCookieConsent()
  const [open, setOpen] = useState(false)
  const [draft, setDraft] = useState(choices || declined)
  const configured = !!(fider.settings.googleAnalytics || fider.settings.googleAdSense)

  useEffect(startGoogle, [])
  useEffect(() => {
    const show = () => {
      setDraft(cookieConsent.snapshot() || declined)
      setOpen(true)
    }
    window.addEventListener("cookie-preferences", show)
    return () => window.removeEventListener("cookie-preferences", show)
  }, [])

  if (!configured || fider.session.props.sponsorPreview) return null

  const save = (value: CookieChoices) => {
    cookieConsent.save(value)
    setOpen(false)
  }

  return <>
    {!choices && !open && (
      <section aria-label="Cookie preferences" className="fixed bottom-4 left-4 right-4 z-overlay mx-auto max-w-3xl rounded-panel border border-border bg-overlay p-4 shadow-xl animate-[windowFadeIn_140ms_var(--ease-out)]">
        {fider.settings.googleAnalytics && (
          <p className="mb-3 mt-0 text-sm">
            Allow Google Analytics cookies to measure visits to pages and posts? <a href="/privacy">Privacy policy</a>
          </p>
        )}
        <div className="flex flex-wrap gap-2">
          {fider.settings.googleAnalytics ? <>
            <Button onClick={() => save({ analytics: true })}>Allow analytics cookies</Button>
            <Button onClick={() => save(declined)}>Reject analytics cookies</Button>
          </> : (
            <Button onClick={() => save(declined)}>Got it</Button>
          )}
        </div>
      </section>
    )}
    <Modal.Window
      isOpen={open}
      onClose={() => setOpen(false)}
      manageHistory={false}
      labelledBy="cookie-preferences-title"
    >
      <Modal.Header><span id="cookie-preferences-title">Cookie preferences</span></Modal.Header>
      <Modal.Content>
        <p className="mt-0 text-sm text-muted">
          Essential cookies keep sign-in and site preferences working.
        </p>
        <div className="space-y-4">
          {fider.settings.googleAnalytics && (
            <label className="flex cursor-pointer items-start gap-3">
              <input
                type="checkbox"
                className="mt-1 cursor-pointer accent-primary"
                checked={draft.analytics}
                onChange={event => setDraft({ ...draft, analytics: event.target.checked })}
              />
              <span>
                <strong className="block">Google Analytics cookies</strong>
                <span className="text-sm text-muted">Measure which pages and posts people visit.</span>
              </span>
            </label>
          )}
        </div>
      </Modal.Content>
      <Modal.Footer>
        {fider.settings.googleAnalytics && <Button onClick={() => save(declined)}>Reject analytics cookies</Button>}
        <Button variant="primary" onClick={() => save(draft)}>Save preferences</Button>
      </Modal.Footer>
    </Modal.Window>
  </>
}
