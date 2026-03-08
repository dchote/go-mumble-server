# UI Style and Guidelines

This document defines the visual design standards and patterns for the go-mumble-server management frontend. All UI should follow these guidelines for consistency.

## Design Philosophy

The app uses a **modern flat palette** with muted olives, charcoal, desaturated gold, and neutral greys. The aesthetic is minimal and professional—avoiding flashy gradients while using subtle depth where appropriate.

## Color Palette

Colors are defined in `frontend/src/styles/theme.scss` and `frontend/src/plugins/vuetify.js`.

- **Primary**: Muted olive
- **Secondary**: Charcoal
- **Accent**: Desaturated gold
- **Background**: Warm light grey
- **Surface**: White
- **Error**: Red for validation and destructive actions

## Typography

- **Font family**: System stack — `-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif`
- **No decorative or custom fonts** — keep the interface clean and readable
- **Card headers**: Use `text-h5` (1.5rem) — avoid `text-h4` which is too large
- **Card titles**: `font-weight: 600`
- Use Vuetify typography classes: `text-h5`, `text-h6`, `text-body-1`, `text-caption`, etc.

## Spacing and Layout

### Content Padding

- Use responsive padding: `pa-3 pa-sm-6` or `pa-3 pa-sm-4` for mobile-first scaling
- Main content: 0 (xs), 20px/16px (600px+), 24px/20px (960px+)

### Border Radius

- Cards: `8px`
- **xs only**: Cards may use `border-radius: 0` for edge-to-edge feel on mobile

## Component Patterns

### App Header

- **Background**: Primary color gradient or solid primary
- **User chip**: Use for username display when authenticated

### Cards

- Use `v-card` with `variant="outlined"` or `variant="flat"` as appropriate
- Add divider under `v-card-title` when using raw v-card

### Buttons

- **Primary actions**: `variant="elevated"`, `color="primary"`
- **Secondary actions**: `variant="outlined"` or `variant="text"`
- **Button groups — REQUIRED**: **NEVER** place adjacent buttons without spacing. Always add `class="mr-2"` to the first/left button(s). Do NOT use `gap`.
- **Form action buttons** (Save, Cancel, Submit): Place at the **bottom** of the form, **right-aligned**. Use `d-flex justify-end mt-4` wrapper.

### Form Inputs

- **All form inputs**: `variant="outlined"`, `density="compact"`, `hide-details="auto"`
- **Login/register**: May use `density="comfortable"` for accessibility
- **autocomplete**: Use `autocomplete="off"` on non-auth fields; use `autocomplete="username"`, `autocomplete="current-password"`, `autocomplete="new-password"` for auth forms
- **Fixed-width inputs**: `style="max-width: 320px;"` (or 200, 400 as appropriate)

### Checkboxes

- **density**: Always use `density="compact"`
- **class**: Add `class="checkbox-compact"` for minimal padding when defined in theme

### Section Spacing

- **Form fields**: Use `mb-4` between fields
- **Action buttons**: Use `mt-4` above the button group
- **Button groups**: Add `class="mr-2"` to the first button
- **Chip groups**: Add `class="mr-2 mb-2"` to each chip

### Feedback and Confirmations

**Never use `alert()` or `confirm()`.** Use in-app UI instead.

- **Success/error feedback**: Use `v-alert` on the parent page. `type="success"` or `type="error"`, `density="compact"`, `class="mb-4"`
- **Confirmations**: Use `v-dialog` with Cancel and primary action buttons

```vue
<v-alert v-if="error" type="error" density="compact" class="mb-4">{{ error }}</v-alert>

<v-dialog v-model="showConfirm" persistent>
  <v-card>
    <v-card-title>Delete item?</v-card-title>
    <v-card-text>Are you sure?</v-card-text>
    <v-card-actions>
      <v-spacer />
      <v-btn variant="text" class="mr-2" @click="showConfirm = false">Cancel</v-btn>
      <v-btn color="error" variant="elevated" @click="doDelete">Delete</v-btn>
    </v-card-actions>
  </v-card>
</v-dialog>
```

### Links

- All links use primary color; never show unstyled blue
- Router links: inherit from global `a` styles

## Mobile Responsiveness

- **Breakpoints**: 0–599px (xs), 600px (sm), 960px (md), 1280px (lg)
- **Responsive padding**: Prefer `pa-3 pa-sm-6` over fixed `pa-6`
- Use `useDisplay()` from Vuetify for programmatic breakpoint checks
- Ensure touch targets are adequately sized (min 44px)

## Do Not

- **Use `gap` or `gap-*`** — use explicit margins (`mr-2`, `mb-2`) instead
- **Place adjacent buttons without spacing** — always add `mr-2` to the first button
- **Use `alert()` or `confirm()`** — use v-alert and v-dialog
- Use raw HTML for forms, buttons, inputs, or cards — always use Vuetify components
- Put form action buttons (Save, Cancel) in the page header — they belong below the form
- Use `hide-details` without `="auto"`
- Use decorative or custom fonts
- Use unstyled blue links
