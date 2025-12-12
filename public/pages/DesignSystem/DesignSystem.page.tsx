import React, { useState } from "react"
import { PostStatus, UserStatus } from "@fider/models"
import {
  PageTitle,
  Button,
  UserName,
  Toggle,
  Avatar,
  ShowTag,
  ShowPostStatus,
  Moment,
  Loader,
  Form,
  Input,
  TextArea,
  RadioButton,
  Select,
  SelectOption,
  ButtonClickEvent,
  Message,
  Hint,
  AvatarStack,
  SocialSignInButton,
  Dropdown,
  Icon,
  Checkbox,
  ImageUploader,
} from "@fider/components"
import { User, UserRole, Tag } from "@fider/models"
import { notify, Failure } from "@fider/services"
import { HStack, VStack } from "@fider/components/layout"
import { heroiconsLightBulb as IconLightBulb, heroiconsSearch as IconSearch } from "@fider/icons.generated"
import { useFider } from "@fider/hooks"

const jonSnow: User = {
  id: 0,
  name: "Jon Snow",
  role: UserRole.Administrator,
  status: UserStatus.Active,
  avatarURL:
    "https://images.unsplash.com/photo-1491528323818-fdd1faba62cc?ixlib=rb-1.2.1&ixqx=1JzWlMeJDF&ixid=eyJhcHBfaWQiOjEyMDd9&auto=format&fit=facearea&facepad=2&w=256&h=256&q=80",
}

const aryaStark: User = {
  id: 0,
  name: "Arya Snow",
  role: UserRole.Visitor,
  status: UserStatus.Active,
  avatarURL: "https://images.unsplash.com/photo-1550525811-e5869dd03032?ixlib=rb-1.2.1&auto=format&fit=facearea&facepad=2&w=256&h=256&q=80",
}

const robStark: User = {
  id: 0,
  name: "Robert Stark",
  role: UserRole.Visitor,
  status: UserStatus.Active,
  avatarURL:
    "https://images.unsplash.com/photo-1500648767791-00dcc994a43e?ixlib=rb-1.2.1&ixqx=1JzWlMeJDF&ixid=eyJhcHBfaWQiOjEyMDd9&auto=format&fit=facearea&facepad=2.25&w=256&h=256&q=80",
}

const easyTag: Tag = { id: 1, slug: "easy", name: "Easy", color: "82c460", isPublic: true }
const normalTag: Tag = { id: 2, slug: "normal", name: "Normal", color: "ebb134", isPublic: false }
const hardTag: Tag = { id: 3, slug: "hard", name: "Hard", color: "9c3630", isPublic: false }
const linkTag: Tag = { id: 4, slug: "link", name: "Link", color: "0991db", isPublic: true }

const visibilityPublic = { label: "Public", value: "public" }
const visibilityPrivate = { label: "Private", value: "private" }

interface ColorSwatchProps {
  name: string
  className: string
}

const ColorSwatch: React.FC<ColorSwatchProps> = ({ name, className }) => (
  <div className="flex flex-col items-center gap-1 w-20">
    <div className={`w-12 h-12 rounded-card border border-border ${className}`} />
    <span className="text-xs text-muted text-center leading-tight">{name}</span>
  </div>
)

const SectionDivider = () => <hr className="border-t border-border my-8" />

const DesignSystemPage = () => {
  const fider = useFider()
  const [error, setError] = useState<Failure | undefined>(undefined)

  const notifyError = async () => {
    notify.error("Something went wrong...")
  }

  const notifySuccess = async () => {
    notify.success("Congratulations! It worked!")
  }

  const notifyStatusChange = (opt?: SelectOption) => {
    if (opt) {
      notify.success(opt.value)
    }
  }

  const showLoading = (e: ButtonClickEvent): void => {
    return e.preventEnable()
  }

  const threeHoursAgo = new Date()
  threeHoursAgo.setHours(threeHoursAgo.getHours() - 3)

  const forceError = async () => {
    setError({
      errors: [
        { field: "title", message: "Title is mandatory" },
        { field: "description", message: "Error #1" },
        { field: "description", message: "Error #2" },
        { field: "status", message: "Status is mandatory" },
        { field: "logo", message: "Logo is mandatory" },
        { field: "gdpr", message: "You have to agree!" },
      ],
    })
  }

  return (
    <div id="p-ui-toolkit" className="page container">
      <h2 className="text-display2 mb-4">1. Semantic Colors</h2>

      <h3 className="text-title mb-3">Surfaces</h3>
      <div className="flex flex-wrap gap-4 mb-6">
        <ColorSwatch name="surface" className="bg-surface" />
        <ColorSwatch name="surface-alt" className="bg-surface-alt" />
        <ColorSwatch name="elevated" className="bg-elevated" />
        <ColorSwatch name="overlay" className="bg-overlay" />
        <ColorSwatch name="tertiary" className="bg-tertiary" />
      </div>

      <h3 className="text-title mb-3">Text</h3>
      <div className="flex flex-wrap gap-4 mb-6">
        <ColorSwatch name="foreground" className="bg-foreground" />
        <ColorSwatch name="muted" className="bg-muted" />
        <ColorSwatch name="subtle" className="bg-subtle" />
      </div>

      <h3 className="text-title mb-3">Borders</h3>
      <div className="flex flex-wrap gap-4 mb-6">
        <ColorSwatch name="border" className="bg-border" />
        <ColorSwatch name="border-strong" className="bg-border-strong" />
      </div>

      <h3 className="text-title mb-3">Actions</h3>
      <div className="flex flex-wrap gap-4 mb-6">
        <ColorSwatch name="primary" className="bg-primary" />
        <ColorSwatch name="primary-hover" className="bg-primary-hover" />
        <ColorSwatch name="secondary" className="bg-secondary" />
        <ColorSwatch name="secondary-hover" className="bg-secondary-hover" />
        <ColorSwatch name="danger" className="bg-danger" />
        <ColorSwatch name="danger-hover" className="bg-danger-hover" />
      </div>

      <h3 className="text-title mb-3">Accent</h3>
      <div className="flex flex-wrap gap-4 mb-6">
        <ColorSwatch name="accent" className="bg-accent" />
        <ColorSwatch name="accent-light" className="bg-accent-light" />
        <ColorSwatch name="accent-medium" className="bg-accent-medium" />
        <ColorSwatch name="accent-dark" className="bg-accent-dark" />
      </div>

      <h3 className="text-title mb-3">Success</h3>
      <div className="flex flex-wrap gap-4 mb-6">
        <ColorSwatch name="success" className="bg-success" />
        <ColorSwatch name="success-light" className="bg-success-light" />
        <ColorSwatch name="success-medium" className="bg-success-medium" />
        <ColorSwatch name="success-dark" className="bg-success-dark" />
      </div>

      <h3 className="text-title mb-3">Warning</h3>
      <div className="flex flex-wrap gap-4 mb-6">
        <ColorSwatch name="warning" className="bg-warning" />
        <ColorSwatch name="warning-light" className="bg-warning-light" />
        <ColorSwatch name="warning-medium" className="bg-warning-medium" />
        <ColorSwatch name="warning-dark" className="bg-warning-dark" />
      </div>

      <h3 className="text-title mb-3">Danger</h3>
      <div className="flex flex-wrap gap-4 mb-6">
        <ColorSwatch name="danger" className="bg-danger" />
        <ColorSwatch name="danger-light" className="bg-danger-light" />
        <ColorSwatch name="danger-medium" className="bg-danger-medium" />
        <ColorSwatch name="danger-dark" className="bg-danger-dark" />
      </div>

      <h3 className="text-title mb-3">Info</h3>
      <div className="flex flex-wrap gap-4 mb-6">
        <ColorSwatch name="info" className="bg-info" />
        <ColorSwatch name="info-light" className="bg-info-light" />
        <ColorSwatch name="info-medium" className="bg-info-medium" />
        <ColorSwatch name="info-dark" className="bg-info-dark" />
      </div>

      <SectionDivider />

      <h2 className="text-display2 mb-4">2. Typography</h2>

      <VStack spacing={2} className="p-4 bg-elevated rounded-card border border-border">
        <span className="text-large">text-large (26px bold)</span>
        <span className="text-display2">text-display2 (22px medium)</span>
        <span className="text-display">text-display (22px semibold)</span>
        <span className="text-header">text-header (22px medium)</span>
        <span className="text-title">text-title (18px medium)</span>
        <span className="text-subtitle">text-subtitle (16px medium)</span>
        <span className="text-body">text-body (16px muted)</span>
        <span className="text-muted">text-muted</span>
        <span className="text-subtle">text-subtle</span>
        <span className="text-category">text-category</span>
        <span className="text-link">text-link</span>
      </VStack>

      <SectionDivider />

      <h2 className="text-display2 mb-4">3. Avatars</h2>

      <VStack>
        <HStack>
          <Avatar user={jonSnow} /> <UserName user={jonSnow} />
        </HStack>
        <HStack>
          <Avatar user={aryaStark} /> <UserName user={aryaStark} />
        </HStack>
        <HStack>
          <AvatarStack users={[jonSnow, aryaStark, robStark]} />
        </HStack>
      </VStack>

      <SectionDivider />

      <h2 className="text-display2 mb-4">4. Page Title</h2>

      <PageTitle title="Page Title" subtitle="This is a page subtitle" />

      <SectionDivider />

      <h2 className="text-display2 mb-4">5. Buttons</h2>

      <VStack>
        <HStack>
          <Button variant="primary" size="large">
            Primary
          </Button>
          <Button size="large">
            <Icon sprite={IconLightBulb} /> <span>Secondary</span>
          </Button>
          <Button size="large">
            <Icon sprite={IconLightBulb} />
          </Button>
          <Button size="large">Secondary</Button>
          <Button variant="tertiary" size="large">
            Tertiary
          </Button>
          <Button variant="danger" size="large">
            Danger
          </Button>
        </HStack>

        <HStack>
          <Button variant="primary">Primary</Button>
          <Button>
            <Icon sprite={IconLightBulb} /> <span>Secondary</span>
          </Button>
          <Button>
            <Icon sprite={IconLightBulb} />
          </Button>
          <Button>Secondary</Button>
          <Button variant="tertiary">Tertiary</Button>
          <Button variant="danger">Danger</Button>
        </HStack>

        <HStack>
          <Button variant="primary" size="small">
            Primary
          </Button>
          <Button size="small">
            <Icon sprite={IconLightBulb} /> <span>Secondary</span>
          </Button>
          <Button size="small">
            <Icon sprite={IconLightBulb} />
          </Button>
          <Button size="small">Secondary</Button>
          <Button variant="tertiary" size="small">
            Tertiary
          </Button>
          <Button variant="danger" size="small">
            Danger
          </Button>
        </HStack>

        <HStack>
          <Button href="#" variant="primary">
            Link
          </Button>
          <Button href="#">
            <Icon sprite={IconLightBulb} /> <span>Link</span>
          </Button>
          <Button href="#">
            <Icon sprite={IconLightBulb} />
          </Button>
          <Button>Link</Button>
          <Button variant="tertiary" href="#">
            Link
          </Button>
          <Button href="#" variant="danger">
            Link
          </Button>
        </HStack>

        <HStack>
          <Button disabled={true} variant="primary">
            Primary
          </Button>
          <Button disabled={true}>
            <Icon sprite={IconLightBulb} /> <span>Secondary</span>
          </Button>
          <Button disabled={true}>
            <Icon sprite={IconLightBulb} />
          </Button>
          <Button disabled={true}>Secondary</Button>
          <Button disabled={true} variant="tertiary">
            Tertiary
          </Button>
          <Button disabled={true} variant="danger">
            Danger
          </Button>
        </HStack>

        <HStack>
          <Button variant="primary" onClick={showLoading}>
            Loading
          </Button>
          <Button onClick={showLoading}>
            <Icon sprite={IconLightBulb} /> <span>Loading</span>
          </Button>
          <Button onClick={showLoading}>
            <Icon sprite={IconLightBulb} />
          </Button>
          <Button onClick={showLoading}>Loading</Button>
          <Button variant="tertiary" onClick={showLoading}>
            Loading
          </Button>
          <Button variant="danger" onClick={showLoading}>
            Loading
          </Button>
        </HStack>

        <HStack className="[&_img]:w-4 [&_img]:h-4">
          <SocialSignInButton option={{ displayName: "GitHub", provider: "github" }} />
          <SocialSignInButton option={{ displayName: "Facebook", provider: "facebook" }} />
          <SocialSignInButton option={{ displayName: "Google", provider: "google" }} />
        </HStack>
      </VStack>

      <SectionDivider />

      <h2 className="text-display2 mb-4">6. Toggle</h2>

      <VStack>
        <Toggle active={true} label="Active" />
        <Toggle active={false} label="Inactive" />
        <Toggle active={true} disabled={true} label="Disabled" />
      </VStack>

      <SectionDivider />

      <h2 className="text-display2 mb-4">7. Statuses</h2>

      <HStack className="flex-wrap">
        <ShowPostStatus status={PostStatus.Open} />
        <ShowPostStatus status={PostStatus.Planned} />
        <ShowPostStatus status={PostStatus.Started} />
        <ShowPostStatus status={PostStatus.Duplicate} />
        <ShowPostStatus status={PostStatus.Completed} />
        <ShowPostStatus status={PostStatus.Declined} />
      </HStack>

      <SectionDivider />

      <h2 className="text-display2 mb-4">8. Tags</h2>

      <VStack>
        <HStack>
          <ShowTag tag={easyTag} />
          <ShowTag tag={normalTag} />
          <ShowTag tag={hardTag} />
          <ShowTag tag={linkTag} link />
        </HStack>
        <HStack>
          <ShowTag tag={easyTag} circular />
          <ShowTag tag={normalTag} circular />
          <ShowTag tag={hardTag} circular />
          <ShowTag tag={linkTag} circular link />
        </HStack>
      </VStack>

      <SectionDivider />

      <h2 className="text-display2 mb-4">9. Notification</h2>

      <HStack>
        <Button onClick={notifySuccess}>Success</Button>
        <Button onClick={notifyError}>Error</Button>
      </HStack>

      <SectionDivider />

      <h2 className="text-display2 mb-4">10. Moment</h2>

      <VStack>
        <div>
          Relative: <Moment locale={fider.currentLocale} date={threeHoursAgo} format="relative" />
        </div>
        <div>
          Short: <Moment locale={fider.currentLocale} date="2017-06-03T16:55:06.815042Z" format="short" />
        </div>
        <div>
          Full: <Moment locale={fider.currentLocale} date="2017-06-03T16:55:06.815042Z" format="full" />
        </div>
      </VStack>

      <SectionDivider />

      <h2 className="text-display2 mb-4">11. Loader</h2>

      <VStack>
        <Loader />
        <Loader text="Loader with text" />
      </VStack>

      <SectionDivider />

      <h2 className="text-display2 mb-4">12. Message</h2>

      <VStack>
        <Message showIcon={true} type="error">
          Something went wrong.
        </Message>
        <Message showIcon={true} type="warning">
          Be careful!
        </Message>
        <Message showIcon={true} type="success">
          Your order has been confirmed.
        </Message>
      </VStack>

      <SectionDivider />

      <h2 className="text-display2 mb-4">13. Hints</h2>

      <VStack>
        <Hint permanentCloseKey="ui-toolkip-example">Did you know that you can close this permanently?</Hint>
        <Hint>You can&apos;t close this one :)</Hint>
      </VStack>

      <SectionDivider />

      <h2 className="text-display2 mb-4">14. Form</h2>

      <Form error={error}>
        <Input label="Title" field="title">
          <p className="text-muted">This is the explanation for the field above.</p>
        </Input>
        <Input label="Disabled!" field="unamed" disabled={true} value={"you can't change this!"} />
        <Input label="Name" field="name" placeholder={"Your name goes here..."} />
        <Input label="Subdomain" field="subdomain" suffix=".fider.io" />
        <Input label="Email" field="email" suffix={<Button variant="primary">Sign in</Button>} />
        <TextArea label="Description" field="description" minRows={5}>
          <p className="text-muted">This textarea resizes as you type.</p>
        </TextArea>
        <Input field="age" placeholder="This field doesn't have a label" />

        <div className="grid gap-4 grid-cols-4">
          <Input label="Title1" field="title1" />
          <Input label="Title2" field="title2" />
          <Input label="Title3" field="title3" />
          <RadioButton label="Visibility" field="visibility" defaultOption={visibilityPublic} options={[visibilityPrivate, visibilityPublic]} />
        </div>

        <Select
          label="Status"
          field="status"
          options={[
            { value: "open", label: "Open" },
            { value: "started", label: "Started" },
            { value: "planned", label: "Planned" },
          ]}
          onChange={notifyStatusChange}
        />

        <ImageUploader label="Logo" field="logo" onChange={console.log} />

        <Checkbox field="gdpr">I agree with this policy.</Checkbox>

        <Button variant="primary" onClick={forceError}>Save</Button>
      </Form>

      <SectionDivider />

      <h2 className="text-display2 mb-4">15. Dropdown</h2>

      <Dropdown renderHandle={<Avatar user={aryaStark} />}>
        <div className="p-2 text-bold uppercase">Arya Stark</div>
        <Dropdown.ListItem>Account Settings</Dropdown.ListItem>
        <Dropdown.ListItem>My Profile</Dropdown.ListItem>
        <Dropdown.Divider />
        <Dropdown.ListItem>Sign Out</Dropdown.ListItem>
      </Dropdown>

      <SectionDivider />

      <h2 className="text-display2 mb-4">16. Search</h2>

      <Input field="search" placeholder="Search..." icon={IconSearch} />

      <SectionDivider />

      <h2 className="text-display2 mb-4">17. Borders</h2>

      <VStack spacing={4}>
        <div className="p-4 border border-border rounded-card">Default border</div>
        <div className="p-4 border border-border-strong rounded-card">Strong border</div>
        <div className="p-4 border border-dashed border-border rounded-card">Dashed border</div>
        <div className="p-4 border border-dashed border-border-strong rounded-card">Dashed strong border</div>
      </VStack>
    </div>
  )
}

export default DesignSystemPage
