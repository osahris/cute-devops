// SPDX-FileCopyrightText: 2023-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
//
// SPDX-License-Identifier: EUPL-1.2

// @ts-check
import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";

// https://astro.build/config
export default defineConfig({
  site: "https://devops.patterns.how",
  trailingSlash: "never",
  build: {
    format: "preserve",
  },
  integrations: [
    starlight({
      title: "Cute DevOps Patterns!",
      logo: {
        src: "./public/emoji_u1f537.svg",
      },
      favicon: "/emoji_u1f537.svg",
      components: {
        SiteTitle: "./src/components/SiteTitle.astro",
      },
      social: {
        github: "https://github.com/mkbrechtel/devops",
      },
      sidebar: [
        { slug: 'index' },
        {
          label: 'Development',
          items: [
            {
              label: 'Frontend',
              autogenerate: { directory: 'development/frontend' },
            },
          ]
        },
        {
          label: 'Operation',
          items: [
            {
              label: 'Deployment',
              autogenerate: { directory: 'operation/deployment' },
            },
          ]
        },
        {
          label: 'Meta',
          autogenerate: { directory: 'meta' },
        },
      ],
      editLink: {
        baseUrl: "https://github.com/mkbrechtel/devops/edit/main/",
      },
    }),
  ],
});
