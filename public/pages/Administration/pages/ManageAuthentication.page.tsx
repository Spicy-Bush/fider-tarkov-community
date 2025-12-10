import React, { useState } from "react"
import { Button, OAuthProviderLogo, Icon, Field, Toggle, Form } from "@fider/components"
import { OAuthConfig, OAuthProviderOption } from "@fider/models"
import { OAuthForm } from "../components/OAuthForm"
import { actions, notify, Fider, Failure } from "@fider/services"
import { heroiconsPlay as IconPlay, heroiconsPencilAlt as IconPencilAlt } from "@fider/icons.generated"
import { HStack, VStack } from "@fider/components/layout"
import { PageConfig } from "@fider/components/layouts"

export const pageConfig: PageConfig = {
  title: "Authentication",
  subtitle: "Manage your site authentication",
  sidebarItem: "authentication",
}

interface ManageAuthenticationPageProps {
  providers: OAuthProviderOption[]
}

const ManageAuthenticationPage: React.FC<ManageAuthenticationPageProps> = (props) => {
  const [isAdding, setIsAdding] = useState(false)
  const [isEmailAuthAllowed, setIsEmailAuthAllowed] = useState(Fider.session.tenant.isEmailAuthAllowed)
  const [canDisableEmailAuth] = useState(() => 
    props.providers.map((o) => o.isEnabled).reduce((a, b) => a || b, false)
  )
  const [editing, setEditing] = useState<OAuthConfig | undefined>()
  const [error, setError] = useState<Failure | undefined>()

  const addNew = async () => {
    setIsAdding(true)
    setEditing(undefined)
  }

  const edit = async (provider: string) => {
    const result = await actions.getOAuthConfig(provider)
    if (result.ok) {
      setEditing(result.data)
      setIsAdding(false)
    } else {
      notify.error("Failed to retrieve OAuth configuration. Try again later")
    }
  }

  const startTest = async (provider: string) => {
    const redirect = `${Fider.settings.baseURL}/oauth/${provider}/echo`
    window.open(`/oauth/${provider}?redirect=${redirect}`, "oauth-test", "width=1100,height=600,status=no,menubar=no")
  }

  const cancel = async () => {
    setIsAdding(false)
    setEditing(undefined)
  }

  const toggleEmailAuth = async (active: boolean) => {
    setIsEmailAuthAllowed(active)
    const response = await actions.updateTenantEmailAuthAllowed(active)
    if (response.ok) {
      notify.success(`You successfully changed email authentication setting.`)
    } else {
      setIsEmailAuthAllowed(!active)
      setError(response.error)
      notify.error("Unable to save this setting.")
    }
  }

  let enabledProvidersCount = 0
  for (const o of props.providers) {
    if (o.isEnabled) {
      enabledProvidersCount++
    }
  }
  const cantDisable = !isEmailAuthAllowed && enabledProvidersCount == 1

  if (isAdding) {
    return <OAuthForm cantDisable={cantDisable} onCancel={cancel} />
  }

  if (editing) {
    return <OAuthForm cantDisable={cantDisable} config={editing} onCancel={cancel} />
  }

  const enabled = <span className="text-green-700">Enabled</span>
  const disabled = <span className="text-red-700">Disabled</span>

  return (
    <VStack spacing={8}>
      <div>
        <h2 className="text-display">General Authentication</h2>
        <Form error={error} className="mt-4">
          <Field label="Allow Email Authentication" className="mt-2">
            <Toggle
              field="isEmailAuthAllowed"
              label={isEmailAuthAllowed ? "Yes" : "No"}
              disabled={!Fider.session.user.isAdministrator || !canDisableEmailAuth}
              active={isEmailAuthAllowed}
              onToggle={toggleEmailAuth}
            />
            {!canDisableEmailAuth && (
              <p className="text-muted my-1">You need to configure another authentication provider before disabling email authentication.</p>
            )}
            <p className="text-muted my-1">
              When email-based authentication is disabled, users will not be allowed to sign in using their email. Thus, they will be forced to use another
              authentication provider, such as your preferred OAuth provider.
            </p>
            <p className="text-muted mt-1">Note: Administrator accounts will still be allowed to sign in using their email.</p>
          </Field>
        </Form>
      </div>
      <div>
        <h2 className="text-display">OAuth Providers</h2>
        <p>
          You can use these section to add any authentication provider thats supports the OAuth2 protocol. Additional information is available in our{" "}
          <a rel="noopener" className="text-link" target="_blank" href="https://fider.io/docs/configuring-oauth/">
            OAuth Documentation
          </a>
          .
        </p>
        <VStack spacing={6}>
          {props.providers.map((o) => (
            <div key={o.provider}>
              <HStack justify="between">
                <HStack className="h-6">
                  <OAuthProviderLogo option={o} />
                  <strong>{o.displayName}</strong>
                </HStack>
                {o.isCustomProvider && (
                  <HStack>
                    {Fider.session.user.isAdministrator && (
                      <Button onClick={() => edit(o.provider)} size="small">
                        <Icon sprite={IconPencilAlt} />
                        <span>Edit</span>
                      </Button>
                    )}
                    <Button onClick={() => startTest(o.provider)} size="small">
                      <Icon sprite={IconPlay} />
                      <span>Test</span>
                    </Button>
                  </HStack>
                )}
              </HStack>
              <div className="text-xs block my-1">{o.isEnabled ? enabled : disabled}</div>
              {o.isCustomProvider && (
                <span className="text-muted">
                  <strong>Client ID:</strong> {o.clientID} <br />
                  <strong>Callback URL:</strong> {o.callbackURL}
                </span>
              )}
            </div>
          ))}
          <div>
            {Fider.session.user.isAdministrator && (
              <Button variant="secondary" onClick={addNew}>
                Add new
              </Button>
            )}
          </div>
        </VStack>
      </div>
    </VStack>
  )
}

export default ManageAuthenticationPage
