# Frontend Patterns Guide

This document defines patterns and best practices for the Vue 3 and Vuetify management frontend. All frontend code should follow these conventions.

## Directory Structure

```
frontend/src/
├── layouts/            # Layout components (AuthenticatedLayout, DefaultLayout)
├── pages/              # Route components (file-based routing)
├── components/         # Reusable UI components
│   └── common/         # Shared layout/form components (BrandCard, StandardDialog, etc.)
├── store/              # Vuex store
│   ├── index.js
│   └── modules/
├── composables/        # Vue 3 composables
├── utils/              # API client, formatters
├── plugins/            # Vuetify, router
├── router/
├── styles/             # Global styles, theme variables
└── App.vue
```

## Layout and Navigation

### List → Detail Pattern

- **Virtual servers**: List (not cards). Click list item → server detail page. Edit button right-aligned in detail header.
- **Channels**: List/table on server detail. Click channel → channel detail page. "Add Channel" button right-aligned in Channels section header.
- **Bans, Registered users**: Lists under section headers. "Add ban" / "Register user" right-aligned in each section header.

### Header Rules

- All header content: `d-flex align-center` (vertically center-aligned).
- Action buttons for tabular data: in the header for that section, right-aligned.
- Use `v-spacer` before right-aligned buttons.

## Components vs Pages Structure

Pages stay minimal and use shared components for layout, forms, and dialogs (mirrors recipe-project):

- **BrandCard** (`components/common/BrandCard.vue`): Page layout card with title, optional header/toolbar/content/actions slots. Gradient header, top border.
- **StandardDialog** (`components/common/StandardDialog.vue`): Modal with header, content, actions. Use for confirmations instead of raw `v-dialog`.
- **BackButton** (`components/common/BackButton.vue`): Icon-only back button for card headers; supports `fallback` route.
- **ListToolbar** (`components/common/ListToolbar.vue`): Toolbar area for filters/controls below card header.

Pages compose these components. Never duplicate card/dialog layouts in pages.

## Layout Structure

- **AuthenticatedLayout**: Used when `isAuthenticated`. App bar, nav drawer, user chip, logout.
- **DefaultLayout**: Used when guest. App bar with Login/Signup CTA.
- App.vue switches layouts based on auth state.

## File-Based Routing (unplugin-vue-router)

- Routes generated from `src/pages/` directory
- `src/pages/index.vue` → `/`
- `src/pages/login.vue` → `/login`
- `src/pages/admin/users.vue` → `/admin/users`
- Dynamic routes: `[slug].vue` → `/:slug`

### Build: Static Imports (importMode: 'sync')

The Vite config uses `importMode: 'sync'` so route components are statically imported. This produces a single JS bundle, avoiding 404s when the frontend is embedded and served by the Go backend.

## Vuetify Component Usage

- **All UI must use Vuetify components** — no raw HTML for forms, buttons, inputs, cards, dialogs
- **Variant**: Use `variant="outlined"` for form inputs
- **Density**: Use `density="compact"` for form inputs (login/register may use `comfortable` for accessibility)
- **Hide details**: Use `hide-details="auto"` on inputs

## Form Input Properties

- `variant="outlined"`
- `density="compact"` (or `comfortable` for login/register)
- `hide-details="auto"`
- **`autocomplete` — REQUIRED on every text field**: Use `autocomplete="off"` on all non-auth fields; use `autocomplete="username"`, `autocomplete="current-password"`, or `autocomplete="new-password"` on auth forms only
- `class="mr-2"` for spacing between inputs
- `style="max-width: 320px;"` for fixed-width inputs

## Form Layout Pattern

```vue
<v-form @submit.prevent="handleSubmit">
  <v-alert v-if="error" type="error" density="compact" class="mb-4">{{ error }}</v-alert>
  <div class="mb-4">
    <v-text-field v-model="name" label="Name" variant="outlined" density="compact" hide-details="auto" autocomplete="off" />
  </div>
  <div class="d-flex justify-end mt-4">
    <v-btn variant="text" class="mr-2" @click="cancel">Cancel</v-btn>
    <v-btn type="submit" color="primary" variant="elevated">Save</v-btn>
  </div>
</v-form>
```

## Feedback and Confirmations

Never use `alert()` or `confirm()`. Use `v-alert` for feedback and **StandardDialog** for confirmations. All dialogs (create, edit, confirm delete, add ban, register user, etc.) must use StandardDialog with title, default slot for content, and `#actions` slot for buttons. Example:

```vue
<StandardDialog v-model="showDeleteDialog" title="Delete user?" max-width="400" :fullscreen="mobile" @close="userToDelete = null">
  <p>Are you sure?</p>
  <template #actions>
    <v-spacer />
    <v-btn variant="text" @click="showDeleteDialog = false">Cancel</v-btn>
    <v-btn color="error" variant="elevated" @click="confirmDelete">Delete</v-btn>
  </template>
</StandardDialog>
```

On mobile, pass `:fullscreen="mobile"` from `useDisplay()` for better UX.

## API and Error Handling

- Centralized API client in `utils/api.js` with JWT in Authorization header
- On 401/403: clear auth, redirect to login
- Catch errors in components and surface via `v-alert`
- Log errors: `console.log('[ComponentName] API error:', error)`

## Spacing Rules

- **Between form fields**: `mb-4`
- **Before action buttons**: `mt-4`
- **Between buttons**: `mr-2` on first button
- **Between chips**: `mr-2 mb-2` on each chip
- **Never use `gap`** — use explicit margins
