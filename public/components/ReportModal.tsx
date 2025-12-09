import React, { useState, useEffect } from "react"
import { Modal, Form, TextArea, Button, Loader } from "@fider/components"
import { Trans } from "@lingui/react/macro"
import { i18n } from "@lingui/core"
import { actions, Failure, classSet } from "@fider/services"
import { ReportType, ReportReason } from "@fider/models"
import "./ReportModal.scss"

interface ReportModalProps {
  isOpen: boolean
  onClose: () => void
  postNumber: number
  commentId?: number
  onSubmit?: () => void
  reasons?: ReportReason[]
}

export const ReportModal: React.FC<ReportModalProps> = ({
  isOpen,
  onClose,
  postNumber,
  commentId,
  onSubmit,
  reasons: propReasons,
}) => {
  const [selectedReason, setSelectedReason] = useState<string>("")
  const [details, setDetails] = useState("")
  const [showDetails, setShowDetails] = useState(false)
  const [reasons, setReasons] = useState<ReportReason[]>(propReasons || [])
  const [isLoading, setIsLoading] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<Failure | undefined>()
  const [success, setSuccess] = useState(false)

  const isComment = commentId !== undefined
  const reportedType: ReportType = isComment ? "comment" : "post"

  useEffect(() => {
    if (isOpen) {
      setSelectedReason("")
      setDetails("")
      setShowDetails(false)
      setError(undefined)
      setSuccess(false)

      if (propReasons && propReasons.length > 0) {
        setReasons(propReasons)
      } else {
        setIsLoading(true)
        actions.getReportReasons().then((result) => {
          if (result.ok) {
            setReasons(result.data)
          }
          setIsLoading(false)
        })
      }
    }
  }, [isOpen, propReasons])

  const handleSubmit = async () => {
    if (!selectedReason) return

    setIsSubmitting(true)
    setError(undefined)

    const result = isComment
      ? await actions.reportComment(postNumber, commentId!, selectedReason, details || undefined)
      : await actions.reportPost(postNumber, selectedReason, details || undefined)

    setIsSubmitting(false)

    if (result.ok) {
      setSuccess(true)
      setTimeout(() => {
        onClose()
        if (onSubmit) onSubmit()
      }, 1500)
    } else {
      setError(result.error)
    }
  }

  const getTitle = () => {
    switch (reportedType) {
      case "post":
        return <Trans id="report.title.post">Report Post</Trans>
      case "comment":
        return <Trans id="report.title.comment">Report Comment</Trans>
      default:
        return <Trans id="report.title.generic">Report</Trans>
    }
  }

  if (success) {
    return (
      <Modal.Window isOpen={isOpen} onClose={onClose} center={true} size="small">
        <Modal.Content>
          <div className="c-report-modal__success">
            <div className="c-report-modal__success-icon">&#10003;</div>
            <p className="c-report-modal__success-title">
              <Trans id="report.success">Report submitted successfully</Trans>
            </p>
            <p className="c-report-modal__success-message">
              <Trans id="report.success.message">
                Thank you for helping keep our community safe.
              </Trans>
            </p>
          </div>
        </Modal.Content>
      </Modal.Window>
    )
  }

  return (
    <Modal.Window isOpen={isOpen} onClose={onClose} center={true} size="small">
      <Modal.Header>{getTitle()}</Modal.Header>

      <Modal.Content>
        <Form error={error}>
          {isLoading ? (
            <div className="py-4 text-center">
              <Loader />
            </div>
          ) : (
            <>
              <div className="mb-4">
                <label className="text-medium mb-2 block">
                  <Trans id="report.reason.label">Why are you reporting this?</Trans>
                </label>
                <div className="c-report-modal__reasons">
                  {reasons.map((reason) => {
                    const buttonClass = classSet({
                      "c-report-modal__reason-button": true,
                      "c-report-modal__reason-button--selected": selectedReason === reason.title,
                    })
                    return (
                      <button
                        key={reason.id}
                        type="button"
                        className={buttonClass}
                        onClick={() => setSelectedReason(reason.title)}
                      >
                        <div className="c-report-modal__reason-title">{reason.title}</div>
                        {reason.description && (
                          <div className="c-report-modal__reason-description">{reason.description}</div>
                        )}
                      </button>
                    )
                  })}
                </div>
              </div>

              {!showDetails && selectedReason && (
                <button
                  type="button"
                  className="text-sm text-link hover:underline mb-4"
                  onClick={() => setShowDetails(true)}
                >
                  <Trans id="report.addDetails">+ Add more details (optional)</Trans>
                </button>
              )}

              {showDetails && (
                <TextArea
                  field="details"
                  label={i18n._("report.details.label", { message: "Additional details" })}
                  value={details}
                  onChange={setDetails}
                  placeholder={i18n._("report.details.placeholder", {
                    message: "Provide any additional context...",
                  })}
                />
              )}
            </>
          )}
        </Form>
      </Modal.Content>

      <Modal.Footer>
        <Button
          variant="primary"
          disabled={!selectedReason || isSubmitting}
          onClick={handleSubmit}
        >
          {isSubmitting ? (
            <Trans id="action.submitting.report">Submitting...</Trans>
          ) : (
            <Trans id="action.submit.report">Submit Report</Trans>
          )}
        </Button>
        <Button variant="tertiary" onClick={onClose}>
          <Trans id="action.cancel">Cancel</Trans>
        </Button>
      </Modal.Footer>
    </Modal.Window>
  )
}
