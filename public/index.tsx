import "@fider/assets/styles/tailwind.css"
// import "@fider/assets/styles/index.scss"

import React from "react"
import { createRoot, Root } from "react-dom/client"
import { ErrorBoundary } from "@fider/components/app/ErrorBoundary"
import { ReadOnlyNotice } from "@fider/components/app/ReadOnlyNotice"
import { DevBanner } from "@fider/components/common/DevBanner"
import { WarningBanner } from "@fider/components/common/WarningBanner"
import { Fider, FiderContext } from "@fider/services/fider"
import * as infraActions from "@fider/services/actions/infra"
import * as push from "@fider/services/push"
import { activateI18N } from "@fider/services/i18n"
import { AccountDraftActivity } from "@fider/components/app/AccountDraftActivity"
import { UserStandingProvider } from "@fider/contexts/UserStandingContext"
import { LayoutProvider } from "@fider/contexts/LayoutContext"

import { I18n } from "@lingui/core"
import { I18nProvider } from "@lingui/react"
import { PageRouter } from "./PageRouter"

if ("serviceWorker" in navigator) {
  push.registerServiceWorker()
}

const logProductionError = (err: Error) => {
  if (Fider.isProduction()) {
    console.error(err)
    infraActions.logError(`react.ErrorBoundary: ${err.message}`, err)
  }
}

const onUnhandledRejection = (evt: PromiseRejectionEvent) => {
  if (evt.reason instanceof Error) {
    infraActions.logError(`window.unhandledrejection: ${evt.reason.message}`, evt.reason)
  } else if (evt.reason) {
    infraActions.logError(`window.unhandledrejection: ${evt.reason.toString()}`)
  }
}

const onWindowError = (evt: ErrorEvent) => {
  if (evt.error && evt.colno > 0 && evt.lineno > 0) {
    infraActions.logError(`window.error: ${evt.message}`, evt.error)
  }
}

window.addEventListener("unhandledrejection", onUnhandledRejection)
window.addEventListener("error", onWindowError)

let root: Root | undefined = import.meta.hot?.data.root
let disposed = false

const bootstrapApp = (i18n: I18n) => {
  if (disposed) {
    return
  }

  const rootElement = document.getElementById("root")
  if (rootElement) {
    root ??= createRoot(rootElement)

    root.render(
      <React.StrictMode>
        <ErrorBoundary onError={logProductionError}>
          <I18nProvider i18n={i18n}>
            <FiderContext.Provider value={fider}>
              <LayoutProvider>
                <UserStandingProvider>
                  <AccountDraftActivity />
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
const fider = Fider.initialize(import.meta.hot?.data.session)
activateI18N(fider.currentLocale).then(bootstrapApp).catch(bootstrapApp)

if (import.meta.hot) {
  import.meta.hot.accept()
  import.meta.hot.dispose((data) => {
    disposed = true
    data.root = root
    data.session = fider.session.getSnapshot()

    window.removeEventListener("unhandledrejection", onUnhandledRejection)
    window.removeEventListener("error", onWindowError)
  })
}
