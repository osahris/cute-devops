// @ts-check
import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";

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
        src: "./public/emoji_u1f4a0.svg",
      },
      favicon: "/emoji_u1f4a0.svg",
      social: {
        github: "https://github.com/mkbrechtel/patterns",
      },
      sidebar: [
        {
          label: "Cute Patterns! 💠",
          link: "/",
        },
        {
          label: "Knowledge",
          autogenerate: { directory: "knowledge" },
        },
        {
          label: "Practices",
          autogenerate: { directory: "practice" },
        },
        {
          label: "Management",
          autogenerate: { directory: "management" },
        },
        {
          label: "Deployment",
          autogenerate: { directory: "deployment" },
        },
      ],
      // editLink: {
      //   baseUrl: 'https://github.com/mkbrechtel/patterns/edit/main/',
      // },
    }),
  ],
});
