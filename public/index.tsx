import "@fider/assets/styles/index.scss"

import React from "react"
import { createRoot } from "react-dom/client"
import { ErrorBoundary, ReadOnlyNotice, DevBanner, WarningBanner, AdminPageLoader } from "@fider/components"
import { classSet, Fider, FiderContext, actions, activateI18N } from "@fider/services"
import { UserStandingProvider } from "@fider/contexts/UserStandingContext"
import { LayoutProvider } from "@fider/contexts/LayoutContext"
import { LayoutResolver } from "@fider/components/layouts"

import { I18n } from "@lingui/core"
import { I18nProvider } from "@lingui/react"
import { AsyncPageLoader } from "./AsyncPages"

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

const bootstrapApp = (i18n: I18n) => {
  document.body.className = classSet({
    "is-authenticated": fider.session.isAuthenticated,
    "is-staff": fider.session.isAuthenticated && fider.session.user.isCollaborator,
    "is-moderator": fider.session.isAuthenticated && fider.session.user.isModerator,
    "is-helper": fider.session.isAuthenticated && fider.session.user.isHelper,
  })

  const rootElement = document.getElementById("root")
  if (rootElement) {
    const root = createRoot(rootElement)
    const isAdminPage = fider.session.page.startsWith("Administration/")

    const pageContent = isAdminPage ? (
      <AdminPageLoader
        initialPageName={fider.session.page}
        initialPageProps={fider.session.props}
      />
    ) : (
      <AsyncPageLoader
        pageName={fider.session.page}
        pageProps={fider.session.props}
        renderWithLayout={(Component, pageConfig, props) => (
          <LayoutResolver
            pageName={fider.session.page}
            pageComponent={Component}
            pageProps={props}
            pageConfig={pageConfig}
          />
        )}
      />
    )

    root.render(
      <React.StrictMode>
        <ErrorBoundary onError={logProductionError}>
          <I18nProvider i18n={i18n}>
            <FiderContext.Provider value={fider}>
              <LayoutProvider>
                <UserStandingProvider>
                  <DevBanner />
                  <WarningBanner />
                  <ReadOnlyNotice />
                  {pageContent}
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
