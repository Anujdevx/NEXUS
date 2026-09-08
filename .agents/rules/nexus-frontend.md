---
description: Strict frontend architecture guidelines for the Nexus project
---

# Nexus Frontend Rules

These constraints must be rigidly followed for any frontend development within the Nexus project to avoid "vibe-coded" AI anti-patterns:

## 1. Styling
- **Tailwind CSS v4 Only**: Do not use legacy config files (`tailwind.config.js` / `.ts`).
- **OKLCH Color Space**: All semantic variables must be defined using raw `oklch()` values to utilize the P3 color gamut and ensure perceptual uniformity. Do not use standard hex or RGB.
- **Anti-Vibe Design Language**: 
  - Strictly avoid generic system fonts (Inter, Roboto, Arial). Use character-rich modern geometric sans (e.g., Space Grotesk, Syne, Clash Display).
  - Avoid predictable purple gradients and beige/sage-green "AI aesthetics".
  - Avoid centered, cookie-cutter 3-column layouts.
  - Implement spatial composition with unexpected layouts, grid-breaking elements, and generous negative space.

## 2. Component Architecture
- **Strict Separation of Concerns**: Keep components extremely modular. Avoid bloat.
- **Data Fetching**: Use `swr` exclusively for lightweight data fetching. Strictly avoid heavy state managers like Redux.
- **Shadcn/UI**: When bringing in Shadcn/UI components, adapt them strictly for Tailwind v4. Do not use legacy `hsl()` CSS variables or wrappers.
- **Responsiveness via Container Queries**: Do not rely exclusively on viewport media queries (`sm:`, `md:`). Use Tailwind v4 Container Queries (`@container`, `@sm:`, `@max-md:`) to ensure context-aware resizing.

## 3. Light/Dark Mode (Anti-FOUC)
- The application must support seamless theming.
- **Light Mode is the Absolute Default**: If no theme is saved in `localStorage`, it must default to `light`, ignoring OS preference.
- Prevent FOUC using a synchronous inline `<script>` injected into the root HTML `<head>`.
- Use the `@custom-variant dark (&:where(.dark, .dark *));` directive in Tailwind.

## 4. Accessibility & Stability
- Mandate semantic HTML tags.
- Provide explicit ARIA labels for all interactive elements.
- Ensure strict error handling with explicit `try-catch` blocks and designated fallback UI states.
