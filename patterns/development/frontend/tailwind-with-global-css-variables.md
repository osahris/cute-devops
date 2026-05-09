---
title: Tailwind CSS with Global Variables Pattern 🎨
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
  <link rel="stylesheet" href="/src/tailwind.css">
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
/* src/tailwind.css */
@import "tailwindcss";

@theme {
  /* Define color theme variables that map to your global variables */
  --color-primary: var(--global-color-primary);
  --color-primary-dark: var(--global-color-primary-dark);

  --color-secondary: var(--global-color-secondary);
  --color-secondary-dark: var(--global-color-secondary-dark);

  --color-accent: var(--global-color-accent);
  --color-accent-dark: var(--global-color-accent-dark);
  
  /* Font configurations */
  --font-sans: var(--global-font-sans);
  --font-serif: var(--global-font-serif); 
  --font-mono: var(--global-font-mono);

}
```

This approach allows Tailwind to generate utility classes based on your theme variables, which in turn reference your global CSS variables.

With Tailwind v4, the theme configuration is done directly in CSS using the `@theme` directive, there is no need for a separate `tailwind.config.js` file anymore.

### 6. Using Theme Variables in Your HTML

 Now you can use these theme variables in your HTML:

```html
<div class="bg-primary text-white dark:bg-primary-dark">
  This uses the primary color from your theme in light mode
  and primary-dark in dark mode.
</div>
```

### 7. Start Your Build Process

Run your build process with Vite:

```bash
npm run dev
```

Go to your browser and visit `http://localhost:3000` to see your application running.

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

## Best Practices 💡
- Limit the number of main colors to maintain design consistency
- Use semantic color names in variables (primary, danger) rather than descriptive ones (blue, red)
- Test themes across different browsers and devices

## Checklist 📋
- [ ] Define base color palette as CSS variables in global.css
- [ ] Load global.css directly in HTML, outside of the build process
- [ ] Install Tailwind CSS with the appropriate method for your project setup
- [ ] Create tailwind CSS file with @import "tailwindcss" and @theme directive
- [ ] Define theme variables that reference global CSS variables
- [ ] Implement dark mode using `dark:`-variant utility classes
- [ ] Document the color system and variable naming conventions
- [ ] Test color contrast for accessibility in both light and dark modes
- [ ] Test across different browsers and devices

## References 📚

For a complete working example of this pattern, check out:
- [hello-tailwind-with-global-variables](https://github.com/mkbrechtel/hello-tailwind-with-global-variables) - A repository demonstrating the implementation of Tailwind CSS with global CSS variables for theming

Tailwind CSS Documentation:
- [Tailwind CSS Theme Variables](https://tailwindcss.com/docs/theme) - Learn about theming in Tailwind v4
- [Dark Mode in Tailwind CSS](https://tailwindcss.com/docs/dark-mode) - Implementation of dark mode using the dark variant
- [Vite Plugin Installation](https://tailwindcss.com/docs/installation/using-vite) - Official guide on installing Tailwind CSS with Vite, also has links to other variants

Astro Tailwind CSS Integration:
- [Astro Tailwind CSS](https://astro.build/integrations/tailwindcss) - Official Astro integration for Tailwind CSS
