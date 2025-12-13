import React from "react"
import { Button } from "@fider/components"
import { actions } from "@fider/services"
import { Trans } from "@lingui/react/macro"

interface APIKeyFormState {
  apiKey?: string
}

export class APIKeyForm extends React.Component<any, APIKeyFormState> {
  constructor(props: any) {
    super(props)
    this.state = {}
  }

  private regenerate = async () => {
    const result = await actions.regenerateAPIKey()
    if (result.ok) {
      this.setState({ apiKey: result.data.apiKey })
    }
  }

  private showAPIKey() {
    return (
      <div className="mt-4 p-4 bg-success-light border border-success-light rounded-card">
        <p className="text-success font-medium mb-2">
          <Trans id="mysettings.apikey.newkey">
            Your new API Key is:
          </Trans>
        </p>
        <code className="block p-3 bg-elevated border border-surface-alt rounded text-sm font-mono break-all">
          {this.state.apiKey}
        </code>
        <p className="text-muted text-sm mt-3">
          <Trans id="mysettings.apikey.newkeynotice">Store it securely on your servers and never store it in the client side of your app.</Trans>
        </p>
      </div>
    )
  }

  public render() {
    return (
      <div>
        <p className="text-muted text-sm mb-3">
          <Trans id="mysettings.apikey.notice">
            The API Key is only shown whenever generated. If your Key is lost or has been compromised, generated a new one and take note of it.
          </Trans>
        </p>
        <p className="text-muted text-sm mb-4">
          <Trans id="mysettings.apikey.documentation">
            To learn how to use the API, read the{" "}
            <a className="text-link" rel="noopener" href="https://fider.io/docs/api" target="_blank">
              official documentation
            </a>
            .
          </Trans>
        </p>
        <Button variant="danger" size="small" onClick={this.regenerate}>
          <Trans id="mysettings.apikey.generate">Regenerate API Key</Trans>
        </Button>
        {this.state.apiKey && this.showAPIKey()}
      </div>
    )
  }
}
