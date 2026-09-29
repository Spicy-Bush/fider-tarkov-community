// SignInControl converted to Tailwind

import React, { useState } from "react"
import { SocialSignInButton } from "@fider/components/common/SocialSignInButton"
import { Form } from "@fider/components/common/form/Form"
import { Button } from "@fider/components/common/Button"
import { Input } from "@fider/components/common/form/Input"
import { Message } from "@fider/components/common/Message"
import { Divider } from "@fider/components/layout/Divider"
import * as device from "@fider/services/device"
import * as tenantActions from "@fider/services/actions/tenant"
import { Failure } from "@fider/services"
import { isCookieEnabled } from "@fider/services/utils"
import { useFider } from "@fider/hooks/use-fider"
import { Trans } from "@lingui/react/macro"

interface SignInControlProps {
  useEmail: boolean
  redirectTo?: string
  onEmailSent?: (email: string) => void
}

export const SignInControl: React.FunctionComponent<SignInControlProps> = (props) => {
  const fider = useFider()
  const [showEmailForm, setShowEmailForm] = useState(fider.session.tenant ? fider.session.tenant.isEmailAuthAllowed : true)
  const [email, setEmail] = useState("")
  const [error, setError] = useState<Failure | undefined>(undefined)

  const forceShowEmailForm = (e: React.MouseEvent<HTMLAnchorElement>) => {
    e.preventDefault()
    setShowEmailForm(true)
  }

  const signIn = async () => {
    const result = await tenantActions.signIn(email)
    if (result.ok) {
      setEmail("")
      setError(undefined)
      if (props.onEmailSent) {
        props.onEmailSent(email)
      }
    } else if (result.error) {
      setError(result.error)
    }
  }

  const providersLen = fider.settings.oauth.length

  if (!isCookieEnabled()) {
    return (
      <Message type="error">
        <h3 className="text-display">Cookies Required</h3>
        <p>Cookies are not enabled on your browser. Please enable cookies in your browser preferences to continue.</p>
      </Message>
    )
  }

  return (
    <div>
      {providersLen > 0 && (
        <>
          <div className="grid grid-cols-3 gap-2 mb-2">
            {fider.settings.oauth.map((o) => (
              <React.Fragment key={o.provider}>
                <SocialSignInButton option={o} redirectTo={props.redirectTo} />
              </React.Fragment>
            ))}
          </div>
          {props.useEmail && <Divider />}
        </>
      )}

      {props.useEmail &&
        (showEmailForm ? (
          <div>
            <p>
              <Trans id="signin.message.email">Enter your email address to sign in</Trans>
            </p>
            <Form error={error}>
              <Input
                field="email"
                value={email}
                autoFocus={!device.isTouch()}
                onChange={setEmail}
                placeholder="yourname@example.com"
                suffix={
                  <Button type="submit" variant="primary" disabled={email === ""} onClick={signIn}>
                    <Trans id="action.signin">Sign in</Trans>
                  </Button>
                }
              />
            </Form>
            {!fider.session.tenant.isEmailAuthAllowed && (
              <p className="text-danger mt-1">
                <Trans id="signin.message.onlyadmins">Currently only allowed to sign in to an administrator account</Trans>
              </p>
            )}
          </div>
        ) : (
          <div>
            <p className="text-muted">
              <Trans id="signin.message.emaildisabled">
                Email authentication has been disabled by an administrator. If you have an administrator account and need to bypass this restriction, please{" "}
                <a href="#" className="font-bold" onClick={forceShowEmailForm}>
                  click here
                </a>
                .
              </Trans>
            </p>
          </div>
        ))}
    </div>
  )
}
