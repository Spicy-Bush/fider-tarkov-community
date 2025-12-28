# Server Development

## Models Naming

- `app/models/cmd.<something>`: Commands that perform actions and potentially return values. E.g.: `cmd.CreateNewUser`, `cmd.SendMail`
- `app/models/query.<something>`: Queries to retrieve data. E.g.: `query.GetUserById`, `query.GetAllPosts`
- `app/models/entity.<something>`: Objects mapped to database tables. E.g.: `entity.User`, `entity.Post`
- `app/models/dto.<something>`: Data transfer objects between packages/services. E.g.: `dto.NewUserInfo`
- `app/models/enum.<something>`: Enumerated types and constants. E.g.: `enum.UserRole`, `enum.PostStatus`

## Actions

Actions in `app/actions/` validate user input for POST/PUT/PATCH requests. Each action maps to a command.

# UI Development

## Build System (Vite)

We use **Vite** for frontend bundling and development.

### Key Configuration

- **Entry point**: `public/index.tsx`
- **Output**: `dist/` directory
- **Public files**: `misc/` folder (copied to `dist/assets/` during build)
- **Base path**: `/assets/` (all JS/CSS served from this path)

The Vite development server is never used, as vite is only there to compile and split the code for the Go backend to serve.

## Folder Structure

```
public/
  assets/styles/       # Global styles (tailwind.css)
  components/          # Shared/reusable components
    common/            # Basic UI components (Button, Input, Modal, etc.)
    app/               # App level components (Header, Footer)
    layouts/           # Layout components (PublicLayout, AdminLayout)
    post/              # Post related components
    moderation/        # Moderation components
  contexts/            # React context providers
  hooks/               # Custom React hooks
  models/              # TypeScript interfaces/types
  pages/               # Page components
  services/            # Business logic, API calls, utilities
    actions/           # API action functions
```

## Page Structure

```
public/pages/Home/
  index.ts             # Exporter
  Home.page.tsx        # Page component
  components/          # Page specific components
```

## Styling

We use **Tailwind CSS** with utility classes.

### CSS Variables

Theme colors are defined in `public/assets/styles/tailwind.css`:

```css
--color-surface: #131415;
--color-foreground: #dbe8e9;
--color-primary: #86b0bc;
--color-danger: #c45a5a;
/* etc. */
```

### Conditional Classes

Use the `classSet` utility for conditional class names:

```tsx
import { classSet } from "@fider/services"

<div className={classSet({
  "bg-primary": isActive,
  "bg-secondary": !isActive,
  "p-4": true,
})} />
```

### Component Styling Pattern

Components use Tailwind classes directly. Variant/size classes are defined as objects:

```tsx
const variantClasses = {
  primary: "text-white bg-primary border border-primary hover:bg-primary-hover",
  secondary: "text-foreground bg-secondary border border-border hover:bg-secondary-hover",
}

const sizeClasses = {
  small: "px-2.5 py-1.5 text-xs",
  default: "px-3 py-2 text-sm",
}
```

## Hooks

Custom hooks live in `public/hooks/` with `use-` or `use` prefix:

- `useFider()` - Access Fider context (session, tenant, settings)
- `usePostFilters()` - Post filtering state and URL sync
- `useNavigate()` - Navigation utilities
- `useCache()` - Client side caching

## Services

Services in `public/services/` handle business logic:

- `services/actions/`       - API calls (e.g., `actions.searchPosts()`)
- `services/http.ts`        - HTTP client wrapper
- `services/fider.ts`       - Fider instance and session
- `services/querystring.ts` - URL query string utilities

## Contexts

React contexts in `public/contexts/` for global state:

- `LayoutContext`       - Layout state management
- `UserStandingContext` - User standing/reputation data
- `UnreadCountsContext` - Notification counts

## Icons

Icons use an SVG sprite system built from individual SVG files.

### Adding a New Icon

1. Add your SVG file to `public/assets/images/` with a descriptive name:
   ```
   public/assets/images/heroicons-my-icon.svg
   ```

2. Run the sprite build script:
   ```bash
   npm run build:sprites
   ```

3. This generates:
   - `misc/sprite.svg` - Combined sprite sheet with all icons as `<symbol>` elements
   - `public/icons.generated.ts` - TypeScript exports for each icon

### Using Icons

```tsx
import { heroiconsCheck as IconCheck } from "@fider/icons.generated"
import { Icon } from "@fider/components"

<Icon sprite={IconCheck} className="h-4 w-4" />
```

## TypeScript

- Interfaces specific to a component stay in the component file
- Shared interfaces go in `public/models/`
- Use strict typing - avoid `any`
