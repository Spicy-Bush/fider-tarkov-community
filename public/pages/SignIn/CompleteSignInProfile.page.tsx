import React, { useState } from "react"
import { Button, Form, Input, LegalFooter } from "@fider/components"
import { actions, Failure } from "@fider/services"
import { i18n } from "@lingui/core"
import { Trans } from "@lingui/react/macro"
import { EmailVerificationKind } from "@fider/models"

interface CompleteSignInProfilePageProps {
  kind: EmailVerificationKind
  k: string
}

const CompleteSignInProfilePage = (props: CompleteSignInProfilePageProps) => {
  const [name, setName] = useState("")
  const [error, setError] = useState<Failure | undefined>()

  const submit = async () => {
    const result = await actions.completeProfile(props.kind, props.k, name)
    if (result.ok) {
      location.href = "/"
    } else if (result.error) {
      setError(result.error)
    }
  }

  return (
    <div id="p-complete-profile">
      <p className="text-title">
        <Trans id="modal.completeprofile.header">Complete your profile</Trans>
      </p>

      <p>
        <Trans id="modal.completeprofile.text">Because this is your first sign in, please enter your name.</Trans>
      </p>
      <Form error={error}>
        <Input
          field="name"
          onChange={setName}
          maxLength={100}
          placeholder={i18n._("modal.completeprofile.name.placeholder", { message: "Name" })}
          suffix={
            <Button type="submit" onClick={submit} variant="primary" disabled={name === ""}>
              <Trans id="action.submit">Submit</Trans>
            </Button>
          }
        />
      </Form>

      <LegalFooter />
    </div>
  )
}

export default CompleteSignInProfilePage
