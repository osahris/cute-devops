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
        src: "./public/emoji_u1f537.svg",
      },
      favicon: "/emoji_u1f537.svg",
      components: {
        SiteTitle: "./src/components/SiteTitle.astro",
      },
      social: {
        github: "https://github.com/mkbrechtel/patterns",
      },
      sidebar: [
        { slug: 'index' },
        // {
        //   label: 'Development',
        //   items: [
        //     {
        //       label: 'Design',
        //       autogenerate: { directory: 'docs/development/design' },
        //     },
        //     {
        //       label: 'Frontend',
        //       autogenerate: { directory: 'docs/development/frontend' },
        //     },
        //     {
        //       label: 'Backend',
        //       autogenerate: { directory: 'docs/development/backend' },
        //     },
        //     {
        //       label: 'Data',
        //       autogenerate: { directory: 'docs/development/data' },
        //     },
        //   ]
        // },
        {
          label: 'Operation',
          items: [
            {
              label: 'Deployment',
              autogenerate: { directory: 'docs/operation/deployment' },
            },
            
          ]
        },
        {
          label: 'Meta',
          autogenerate: { directory: 'docs/meta' },
        },
      ],
      editLink: {
        baseUrl: "https://github.com/mkbrechtel/patterns/edit/main/",
      },
    }),
  ],
});
