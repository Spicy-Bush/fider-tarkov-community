import React from "react"
import { Icon } from "@fider/components"
import { ccVisa as IconVisa, ccAmex as IconAMEX, ccDiners as IconDiners, ccDiscover as IconDiscover, ccJcb as IconJCB, ccMaestro as IconMaestro, ccMastercard as IconMasterCard, ccUnionpay as IconUnionPay, ccGeneric as IconGeneric } from "@fider/icons.generated"
import { HStack } from "@fider/components/layout"

interface CardDetailsProps {
  cardType: string
  lastFourDigits: string
  expiryDate: string
}

const brands: { [key: string]: SpriteSymbol } = {
  visa: IconVisa,
  master: IconMasterCard,
  american_express: IconAMEX,
  discover: IconDiscover,
  jcb: IconJCB,
  maestro: IconMaestro,
  diners_club: IconDiners,
  unionpay: IconUnionPay,
}

export const CardDetails = (props: CardDetailsProps) => {
  const icon = brands[props.cardType] || IconGeneric

  return (
    <HStack>
      <Icon sprite={icon} className="h-6" />
      <span>{props.lastFourDigits}</span>
      <span>Exp. {props.expiryDate}</span>
    </HStack>
  )
}
