import React, { useEffect } from "react"
import { navigator } from "@fider/services"
import { Icon } from "@fider/components"
import { heroiconsXCircle as IconXCircle, heroiconsCheckCircle as IconCheckCircle, heroiconsExclamation as IconExclamation } from "@fider/icons.generated"
import { HStack, VStack } from "@fider/components/layout"

interface OAuthEchoPageProps {
  err: string | undefined
  body: string
  profile: {
    id: string
    name: string
    email: string
  }
}

const ok = <Icon sprite={IconCheckCircle} className="h-4 text-success" />
const error = <Icon sprite={IconXCircle} className="h-4 text-danger" />
const warn = <Icon sprite={IconExclamation} className="h-4 text-warning" />

const OAuthEchoPage: React.FC<OAuthEchoPageProps> = (props) => {
  useEffect(() => {
    navigator.replaceState("/")
  }, [])

  const renderError = () => {
    return (
      <>
        <h5 className="text-display">Error</h5>
        <pre>{props.err}</pre>
      </>
    )
  }

  const renderParseResult = () => {
    const idOk = props.profile && props.profile.id !== ""
    const nameOk = props.profile && props.profile.name !== "Anonymous"
    const emailOk = props.profile && props.profile.email !== ""

    let responseBody = ""
    try {
      responseBody = JSON.stringify(JSON.parse(props.body), null, "  ")
    } catch {
      responseBody = props.body
    }

    return (
      <>
        <h5 className="text-display mb-2">Raw Body</h5>
        <pre className="text-sm overflow-auto">{responseBody}</pre>
        <h5 className="text-display mb-2 mt-8">Parsed Profile</h5>
        <VStack divide={true} spacing={2}>
          <VStack>
            <HStack>
              {idOk ? ok : error}
              <strong>ID:</strong> <span>{props.profile && props.profile.id}</span>
            </HStack>
            {!idOk && <span className="text-muted">ID is required. If not found, users will see an error during sign in process.</span>}
          </VStack>
          <VStack>
            <HStack>
              {nameOk ? ok : warn}
              <strong>Name:</strong> <span>{props.profile && props.profile.name}</span>
            </HStack>
            {!nameOk && (
              <span className="text-muted">
                Name is required, if not found we&apos;ll use <strong>Anonymous</strong> as the name of every new user.
              </span>
            )}
          </VStack>
          <VStack>
            <HStack>
              {emailOk ? ok : warn}
              <strong>Email:</strong> {props.profile && props.profile.email}
            </HStack>
            {!emailOk && (
              <span className="text-muted">
                Email is not required, but highly recommended. If invalid or not found, new users won&apos;t receive notifications.
              </span>
            )}
          </VStack>
        </VStack>
      </>
    )
  }

  return (
    <div id="p-oauth-echo">
      {props.err ? renderError() : renderParseResult()}
    </div>
  )
}

export default OAuthEchoPage
