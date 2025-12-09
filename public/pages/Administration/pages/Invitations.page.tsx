import React, { useState } from "react"
import { Button, TextArea, Form, Input, Field } from "@fider/components"
import { actions, notify, Failure, Fider } from "@fider/services"
import { PageConfig } from "@fider/components/layouts"

export const pageConfig: PageConfig = {
  title: "Invitations",
  subtitle: "Invite people to share their feedback",
  sidebarItem: "invitations",
}

const InvitationsPage: React.FC = () => {
  const [subject, setSubject] = useState(`[${Fider.session.tenant.name}] We would like to hear from you!`)
  const [message, setMessage] = useState(`Hi,

We are inviting you to join the ${Fider.session.tenant.name} feedback site, a place where you can vote, discuss and share your ideas and thoughts on how to improve our services!

Click the link below to join!

%invite%

Regards,
${Fider.session.user.name} (${Fider.session.tenant.name})`)
  const [recipients, setRecipients] = useState<string[]>([])
  const [numOfRecipients, setNumOfRecipients] = useState(0)
  const [rawRecipients, setRawRecipients] = useState("")
  const [error, setError] = useState<Failure | undefined>()

  const handleSetRecipients = (raw: string) => {
    const list = raw.split(/\n|;|,|\s/gm).filter((x) => !!x)
    setRawRecipients(raw)
    setRecipients(list)
    setNumOfRecipients(list.length)
  }

  const sendSample = async () => {
    const result = await actions.sendSampleInvite(subject, message)
    if (result.ok) {
      notify.success(
        <span>
          An email message was sent to <strong>{Fider.session.user.email}</strong>
        </span>
      )
    }
    setError(result.error)
  }

  const sendInvites = async () => {
    const result = await actions.sendInvites(subject, message, recipients)
    if (result.ok) {
      notify.success("Your invites have been sent.")
      setRawRecipients("")
      setNumOfRecipients(0)
      setRecipients([])
      setError(undefined)
    } else {
      setError(result.error)
    }
  }

  return (
    <Form error={error}>
      <TextArea
        field="recipients"
        label="Send invitations to"
        placeholder="james@example.com; mary@example.com"
        minRows={1}
        value={rawRecipients}
        onChange={handleSetRecipients}
      >
        <div className="text-muted">
          <p>
            Input the list of all email addresses you wish to invite. Separate each address with either <strong>semicolon</strong>, <strong>comma</strong>,{" "}
            <strong>whitespace</strong> or <strong>line break</strong>.
          </p>
          <p>You can send this invite to a maximum of 30 recipients each time.</p>
        </div>
      </TextArea>

      <Input field="subject" label="Subject" value={subject} maxLength={70} onChange={setSubject}>
        <p className="text-muted">This is the subject that will be used on the invitation email. Keep it short and sweet.</p>
      </Input>

      <TextArea field="message" label="Message" minRows={8} value={message} onChange={setMessage}>
        <div className="text-muted">
          <p>
            This is the content of the invite. Be polite and explain what this invite is for, otherwise there&apos;s a high change people will ignore your
            message.
          </p>
          <p>
            You&apos;re allowed to write whatever you want as long as you include the invitation link placeholder named <strong>%invite%</strong>.
          </p>
        </div>
      </TextArea>

      <Field label="Sample Invite">
        {Fider.session.user.email ? (
          <Button onClick={sendSample}>Send a sample email to {Fider.session.user.email}</Button>
        ) : (
          <Button disabled={true}>Your profile doesn&apos;t have an email</Button>
        )}
      </Field>

      <Field label="Confirmation">
        <p className="text-muted">Whenever you&apos;re ready, click the following button to send out these invites.</p>
        <Button onClick={sendInvites} variant="primary" disabled={numOfRecipients === 0}>
          Send {numOfRecipients} {numOfRecipients === 1 ? "invite" : "invites"}
        </Button>
      </Field>
    </Form>
  )
}

export default InvitationsPage
