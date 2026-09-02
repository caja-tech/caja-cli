import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import mermaid from 'astro-mermaid';

export default defineConfig({
  integrations: [
    mermaid(),
    starlight({
      title: 'Project Specification',
      sidebar: [
        {
          label: 'Start Here',
          items: [
            { label: 'Introduction', link: '/intro/' },
            { label: 'Project Folder Structure', link: '/structure/' },
            { label: 'CLI Commands', link: '/cli/' },
          ],
        },
        {
          label: 'Language',
          items: [
            { label: 'Language Specification', link: '/language/spec/' },
            { label: 'Lexer', link: '/language/lexer/' },
            { label: 'Parser', link: '/language/parser/' },
            { label: 'Semantic Analyzer', link: '/language/semantic/' },
            { label: 'Abstract Syntax Tree', link: '/language/ast/' },
            { label: 'Interpreter', link: '/language/interpreter/' },
            { label: 'Environment', link: '/language/environment/' },
            { label: 'Compiler (Future)', link: '/language/compiler/' },
            { label: 'Language Server Protocol', link: '/language/lsp/' },
          ],
        },
        {
          label: 'Builtin Modules',
          items: [
            { label: 'Math', link: '/modules/math/' },
            { label: 'String', link: '/modules/string/' },
            { label: 'Array', link: '/modules/array/' },
            { label: 'Map', link: '/modules/map/' },
            { label: 'Date', link: '/modules/date/' },
            { label: 'Cast', link: '/modules/cast/' },
            { label: 'Log', link: '/modules/log/' },
          ],
        },
        {
          label: 'Tooling',
          items: [
            { label: 'VS Code Extensions', link: '/tooling/vscode/' },
          ],
        },
      ],
    }),
  ],
  legacy: { collections: true }
});
