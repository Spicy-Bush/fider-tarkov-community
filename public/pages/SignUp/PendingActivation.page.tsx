import React from "react"
import { Trans } from "@lingui/react/macro"

const PendingActivation = () => {
  return (
    <div id="p-pending-activation" className="text-center">
      <h1 className="text-display uppercase">
        <Trans id="page.pendingactivation.title">Your account is pending activation</Trans>
      </h1>
      <p>
        <Trans id="page.pendingactivation.text">We sent you a confirmation email with a link to activate your site.</Trans>
      </p>
      <p>
        <Trans id="page.pendingactivation.text2">Please check your inbox to activate it.</Trans>
      </p>
    </div>
  )
}

export default PendingActivation
