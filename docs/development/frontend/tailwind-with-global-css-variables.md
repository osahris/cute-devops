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

First, create a dedicated CSS file for your global variables. This file should be loaded at runtime and independent of Tailwind:

```css
/* variables.css */
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

Include this file in your HTML before your main stylesheet:

```html
<link rel="stylesheet" href="variables.css">
<link rel="stylesheet" href="main.css">
```

### 2. Initialize Tailwind CSS

In your main CSS file, import Tailwind's directives:

```css
/* main.css */
@tailwind base;
@tailwind components;
@tailwind utilities;
```

This separation ensures that your variables are defined independently from Tailwind's compilation process, giving you more flexibility for theming.

### 3. Configure Tailwind

Extend your Tailwind configuration to use these variables:

```javascript
// tailwind.config.js
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

### 4. Implement Theme Switching

For dark mode or theme switching, add media query or class-based overrides:

```css
/* For automatic dark mode */
@media (prefers-color-scheme: dark) {
  :root {
    --color-text: #f3f4f6;
    --color-background: #111827;
    /* Adjust other colors as needed */
  }
}

/* For class-based theme switching */
.dark-theme {
  --color-text: #f3f4f6;
  --color-background: #111827;
  /* Other color adjustments */
}
```

### 5. Additional Approaches (Optional)

For more dynamic theming, you can manipulate CSS variables with JavaScript:

```javascript
// Example: Dynamically change theme at runtime
document.documentElement.style.setProperty('--color-primary', '#8B5CF6');
```

You can also load theme configurations from external sources:

```javascript
// Load theme from external source
async function loadUserTheme() {
  const response = await fetch('/api/user-theme');
  const theme = await response.json();

  Object.entries(theme).forEach(([key, value]) => {
    document.documentElement.style.setProperty(`--color-${key}`, value);
  });
}
```

For global site configurations, you can load a separate CSS file based on site settings:

```javascript
// Load a specific theme CSS file
function loadThemeFile(themeName) {
  const link = document.createElement('link');
  link.rel = 'stylesheet';
  link.href = `/themes/${themeName}.css`;
  document.head.appendChild(link);
}

// Example usage
loadThemeFile('corporate-blue');
```

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
- [ ] Define base color palette as CSS variables
- [ ] Configure Tailwind to use CSS variables
- [ ] Create theme variations (light/dark)
- [ ] Test color contrast for accessibility
- [ ] Document the color system
- [ ] Create example components showcasing the theming system
- [ ] Set up a theme toggle mechanism if needed
- [ ] Test across different browsers and devices

## Related Patterns 🔗
- [Cuteness Pattern 🌸](../practice/cuteness.md) - For creating approachable UI experiences