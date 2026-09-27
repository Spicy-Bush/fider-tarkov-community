import "@fider/assets/styles/tailwind.css"
// import "@fider/assets/styles/index.scss"

import React, { useEffect } from "react"
import { createRoot } from "react-dom/client"
import { ErrorBoundary, ReadOnlyNotice, DevBanner, WarningBanner } from "@fider/components"
import { classSet, Fider, FiderContext, actions, activateI18N, push } from "@fider/services"
import { UserStandingProvider } from "@fider/contexts/UserStandingContext"
import { LayoutProvider } from "@fider/contexts/LayoutContext"

import { I18n } from "@lingui/core"
import { I18nProvider } from "@lingui/react"
import { PageRouter } from "./PageRouter"
import { useCurrentUser } from "@fider/hooks"

if ("serviceWorker" in navigator) {
  push.registerServiceWorker()
}

const logProductionError = (err: Error) => {
  if (Fider.isProduction()) {
    console.error(err)
    actions.logError(`react.ErrorBoundary: ${err.message}`, err)
  }
}

window.addEventListener("unhandledrejection", (evt: PromiseRejectionEvent) => {
  if (evt.reason instanceof Error) {
    actions.logError(`window.unhandledrejection: ${evt.reason.message}`, evt.reason)
  } else if (evt.reason) {
    actions.logError(`window.unhandledrejection: ${evt.reason.toString()}`)
  }
})

window.addEventListener("error", (evt: ErrorEvent) => {
  if (evt.error && evt.colno > 0 && evt.lineno > 0) {
    actions.logError(`window.error: ${evt.message}`, evt.error)
  }
})

const SessionAppearance = () => {
  const user = useCurrentUser()

  useEffect(() => {
    document.body.className = classSet({
      "is-authenticated": !!user,
      "is-staff": !!user?.isCollaborator,
      "is-moderator": !!user?.isModerator,
      "is-helper": !!user?.isHelper,
    })
  }, [user])

  return null
}

const bootstrapApp = (i18n: I18n) => {
  const rootElement = document.getElementById("root")
  if (rootElement) {
    const root = createRoot(rootElement)

    root.render(
      <React.StrictMode>
        <ErrorBoundary onError={logProductionError}>
          <I18nProvider i18n={i18n}>
            <FiderContext.Provider value={fider}>
              <LayoutProvider>
                <UserStandingProvider>
                  <SessionAppearance />
                  <DevBanner />
                  <WarningBanner />
                  <ReadOnlyNotice />
                  <PageRouter initialPageName={fider.session.page} />
                </UserStandingProvider>
              </LayoutProvider>
            </FiderContext.Provider>
          </I18nProvider>
        </ErrorBoundary>
      </React.StrictMode>
    )
  }
}
const fider = Fider.initialize()
activateI18N(fider.currentLocale).then(bootstrapApp).catch(bootstrapApp)
