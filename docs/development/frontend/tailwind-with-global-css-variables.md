---
title: Tailwind CSS with Global Variables Pattern 🎨
# sidebar:
#   hidden: true
---
## Overview 📋
The Tailwind CSS with Global Variables Pattern offers a maintainable approach to theme management by combining Tailwind's utility-first methodology with CSS custom properties (variables). This allows for consistent theming across applications while maintaining the flexibility to change colors without recompiling Tailwind.

## Goals 🎯
- Create a consistent color system throughout the application
- Enable easy theme switching (light/dark mode, multiple themes)
- Maintain Tailwind CSS's utility-first benefits
- Centralize color definitions for better maintainability
- Provide a flexible system that can evolve with design needs

## Implementation 🛠️

### 1. Define Global CSS Variables

Create a CSS file with your global variables:

```css
/* public/global.css */
:root {
  /* Colors in light mode */
  --global-color-primary: #60a5fa;
  --global-color-primary-dark: #2563eb;

  --global-color-secondary: #34d399;
  --global-color-secondary-dark: #059669;

  --global-color-accent: #fbbf24;
  --global-color-accent-dark: #d97706;

  --global-color-text: #1f2937;
  --global-color-background: #ffffff;

  /* Dark mode colors */
  --global-color-text-dark: #f3f4f6;
  --global-color-background-dark: #111827;

  /* Fonts */
  --global-font-sans: ui-sans-serif, system-ui, sans-serif,;
  --global-font-serif: ui-serif;
  --global-font-mono: ui-monospace, monospace;
}
```
> Note: The variables are prefixed with `--global-` here to indicate their usage across the entire application. You could use another prefix like `--site-`, `--project-`, or your project name (e.g., `--hello-`). Choosing a distinctive prefix prevents potential naming conflicts with third-party libraries or framework-specific variables.

### 2. Load Global CSS Variables at Runtime

To ensure the global variables are available at runtime and not part of the Vite build process, include the global.css file directly in your HTML file:

```html
<!-- index.html -->
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>My Application</title>
  <!-- Load global CSS variables directly -->
  <link rel="stylesheet" href="/global.css">
  <!-- Then load your compiled Tailwind CSS -->
  <link rel="stylesheet" href="/tailwind.css">
</head>
<body>
  <!-- Your application content -->
</body>
</html>
```

This approach ensures the global variables are loaded independently from the build process and can be easily modified without recompiling Tailwind.

### 3. Install Tailwind CSS with Vite Plugin

With Tailwind CSS v4, the integration with Vite is more streamlined using the official plugin:

```bash
npm install tailwindcss @tailwindcss/vite
```

> **Note:** The actual Tailwind integration depends on the framework you're using. For example, Astro has its own [Astro Tailwind Plugin](https://github.com/withastro/astro/tree/main/packages/astro-tailwind-plugin), and other frameworks may have specific integration methods.

### 4. Configure the Vite Plugin

Add the `@tailwindcss/vite` plugin to your Vite configuration:

```javascript
// vite.config.js or vite.config.ts
import { defineConfig } from 'vite'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [
    tailwindcss(),
  ],
})
```

### 5. Create Tailwind CSS Main File

Create a main CSS file that imports Tailwind and define your theme variables:

```css
/* tailwind.css */
@import "tailwindcss";

@theme {
  /* Define color theme variables that map to your global variables */
  --color-primary: var(--global-color-primary);
  --color-primary-dark: var(--global-color-primary-dark);

  --color-secondary: var(--global-color-secondary);
  --color-secondary-dark: var(--global-color-secondary-dark);

  --color-accent: var(--global-color-accent);
  --color-accent-dark: var(--global-color-accent-dark);

  /* Configure dark mode to use dark variables */
}

/* Configure dark mode variant to use class selector for manual toggling */
@custom-variant dark (&:where(.dark, .dark *));
```

This approach allows Tailwind to generate utility classes based on your theme variables, which in turn reference your global CSS variables.

### 6. Using Theme Variables in Your HTML

With Tailwind v4, the theme configuration is done directly in CSS using the `@theme` directive. Now you can use these theme variables in your HTML:

```html
<div class="bg-primary text-white dark:bg-primary-dark">
  This uses the primary color from your theme in light mode
  and primary-dark in dark mode.
</div>
```

You can toggle dark mode by adding the `dark` class to an ancestor element, typically the html or body tag:

```html
<html class="dark">
  <!-- This will activate dark mode for the entire page -->
</html>
```

### 7. Start Your Build Process

Run your build process with Vite:

```bash
npm run dev
```

## Additional Approaches (Optional)


### Dynamic Color Switching
For more dynamic theming, you can manipulate CSS variables with JavaScript:

```javascript
// Example: Dynamically change theme at runtime
document.documentElement.style.setProperty('--global-color-primary', '#8B5CF6');
```

### Site-wide Theme Configuration
For global sites with multiple services, you can centralize your theming by referencing CSS variables in your HTML. This ensures consistent branding across all your services without duplicating theme configurations on each service.

```html
<!-- Link to your central theme stylesheet in the head section -->
<head>
  <link rel="stylesheet" href="https://style.example.org/themes/corporate-blue.css">
</head>

<!-- Using theme variables in HTML components -->
<div class="bg-primary text-background p-4">
  <h2 class="text-accent-light">Welcome to Our Platform</h2>
  <p>This component uses our centralized theme colors.</p>
</div>
```

This approach allows all services to automatically receive theme updates when the central theme is updated, maintaining visual consistency across your ecosystem.

## Security Considerations 🔐
- CSS variables are client-side, so don't store sensitive information in them
- Ensure contrast ratios meet WCAG accessibility standards for all themes

## Anti-patterns ⚠️
- ❌ Hardcoding colors throughout your templates instead of using theme classes
- ❌ Creating too many color variations, making the system hard to maintain
- ❌ Bypassing the pattern for "quick fixes" that lead to inconsistency
- ❌ Neglecting to test color contrast in different themes
- ❌ Forgetting to document the color system for other developers

## Best Practices 💡
- Create a color palette documentation page showing all available colors
- Limit the number of main colors to maintain design consistency
- Use semantic color names in variables (primary, danger) rather than descriptive ones (blue, red)
- Set up automated testing for color contrast accessibility
- Consider design tokens as a more comprehensive approach for larger systems
- Test themes across different browsers and devices

## Checklist 📋
- [ ] Define base color palette as CSS variables in global.css
- [ ] Load global.css directly in HTML, outside of the build process
- [ ] Install Tailwind CSS and Vite plugin
- [ ] Configure Vite to use Tailwind plugin
- [ ] Configure Tailwind to use CSS variables
- [ ] Create theme variations (light/dark) as separate CSS files
- [ ] Test color contrast for accessibility
- [ ] Document the color system
- [ ] Create example components showcasing the theming system
- [ ] Set up a theme toggle mechanism if needed
- [ ] Test across different browsers and devices

## References 📚

For a complete working example of this pattern, check out:
- [hello-tailwind-with-global-variables](https://github.com/mkbrechtel/hello-tailwind-with-global-variables) - A repository demonstrating the implementation of Tailwind CSS with global CSS variables for theming
