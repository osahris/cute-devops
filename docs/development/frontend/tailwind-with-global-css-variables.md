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
  --color-primary: #3b82f6;
  --color-primary-light: #60a5fa;
  --color-primary-dark: #2563eb;

  --color-secondary: #10b981;
  --color-secondary-light: #34d399;
  --color-secondary-dark: #059669;

  --color-accent: #f59e0b;
  --color-accent-light: #fbbf24;
  --color-accent-dark: #d97706;

  --color-text: #1f2937;
  --color-background: #ffffff;
}
```

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
  <link rel="stylesheet" href="/main.css">
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

> **Note:** The actual Tailwind integration depends on the framework you're using. For example, Astro has its own [Astro Tailwind Plugin](https://github.com/withastro/astro/tree/main/packages/astro-tailwind-plugin), and other frameworks may have specific integration methods.

### 5. Create Tailwind CSS Main File

Create a main CSS file that imports only Tailwind (without the variables):

```css
/* main.css */
@import "tailwindcss";
```

Note that we're not importing global variables here since they will be loaded directly at runtime.

### 6. Configure Tailwind

Extend your Tailwind configuration to use the runtime CSS variables:

```javascript
// tailwind.config.js
/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./src/**/*.{html,js,jsx,ts,tsx}"],
  theme: {
    extend: {
      colors: {
        primary: {
          DEFAULT: 'var(--color-primary)',
          light: 'var(--color-primary-light)',
          dark: 'var(--color-primary-dark)',
        },
        secondary: {
          DEFAULT: 'var(--color-secondary)',
          light: 'var(--color-secondary-light)',
          dark: 'var(--color-secondary-dark)',
        },
        accent: {
          DEFAULT: 'var(--color-accent)',
          light: 'var(--color-accent-light)',
          dark: 'var(--color-accent-dark)',
        },
        text: 'var(--color-text)',
        background: 'var(--color-background)',
      },
    },
  },
  plugins: [],
}
```

### 7. Start Your Build Process

Run your build process with Vite:

```bash
npm run dev
```

### 8. Implement Theme Switching

For dark mode or theme switching, create separate theme CSS files that override the variables, and load them as needed:

```css
/* dark-theme.css */
:root {
  --color-text: #f3f4f6;
  --color-background: #111827;
  /* Adjust other colors as needed */
}
```

You can then dynamically load this theme file at runtime:

```javascript
// Toggle theme function
function toggleDarkMode() {
  const darkThemeLink = document.getElementById('dark-theme');

  if (darkThemeLink) {
    // Remove dark theme if it exists
    darkThemeLink.remove();
  } else {
    // Add dark theme if it doesn't exist
    const link = document.createElement('link');
    link.id = 'dark-theme';
    link.rel = 'stylesheet';
    link.href = '/dark-theme.css';
    document.head.appendChild(link);
  }
}
```

## Additional Approaches (Optional)

For more dynamic theming, you can manipulate CSS variables with JavaScript:

### Dynamic Color Switching
```javascript
// Example: Dynamically change theme at runtime
document.documentElement.style.setProperty('--color-primary', '#8B5CF6');
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
