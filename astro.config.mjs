// @ts-check
import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";

import fs from "fs";
import path from "path";

const docsPath = "./docs/";

const topSidebar = [
  {
    label: "Cute Patterns! 🔷",
    link: "/",
  },
];

// Get all category directories
function getCategories() {
  return fs
    .readdirSync(docsPath)
    .filter((file) => fs.statSync(path.join(docsPath, file)).isDirectory());
}

// Get all markdown files in a directory
/**
 * @param {string} dir
 */
function getMdFiles(dir) {
  return fs
    .readdirSync(dir)
    .filter((file) => file.endsWith(".md"))
    .map((file) => path.basename(file, ".md"));
}

// Create hierarchical menu with category groups
export function createSidebar() {
  // Get categories
  const categories = getCategories();

  // Create menu structure
  const categorySidebar = categories.map((category) => ({
    label: category.charAt(0).toUpperCase() + category.slice(1),
    items: getMdFiles(path.join(docsPath, category)).map(
      (file) => `${category}/${file}`,
    ),
  }));
  return [...topSidebar, ...categorySidebar];
}

// https://astro.build/config
export default defineConfig({
  site: "https://patterns.mkbrechtel.dev",
  trailingSlash: "never",
  build: {
    format: "preserve",
  },
  integrations: [
    starlight({
      title: "Cute Patterns!",
      logo: {
        src: "./public/emoji_u1f537.svg",
      },
      favicon: "/emoji_u1f537.svg",
      components: {
        SiteTitle: "./src/components/SiteTitle.astro",
      },
      social: {
        github: "https://github.com/mkbrechtel/patterns",
      },
      sidebar: createSidebar(),
      editLink: {
        baseUrl: "https://github.com/mkbrechtel/patterns/edit/main/",
      },
    }),
  ],
});
