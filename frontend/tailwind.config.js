/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    // Align Tailwind's built-in text-* classes with the design system
    fontSize: {
      xs:   ['var(--text-xs)',  { lineHeight: 'var(--lh-xs)' }],   // 11px
      sm:   ['var(--text-sm)',  { lineHeight: 'var(--lh-sm)' }],   // 13px
      base: ['var(--text-base)', { lineHeight: 'var(--lh-base)' }], // 14px
      lg:   ['var(--text-lg)',  { lineHeight: 'var(--lh-lg)' }],   // 16px
      xl:   ['var(--text-xl)',  { lineHeight: 'var(--lh-xl)' }],   // 20px
      '2xl': ['1.5rem', { lineHeight: '2rem' }],
      '3xl': ['1.875rem', { lineHeight: '2.25rem' }],
    },
    extend: {
      typography: {
        DEFAULT: {
          css: {
            lineHeight: '1.6',
            p:    { marginTop: '0.4em', marginBottom: '0.4em' },
            li:   { marginTop: '0.15em', marginBottom: '0.15em' },
            h1:   { marginTop: '0.9em', marginBottom: '0.4em' },
            h2:   { marginTop: '0.8em', marginBottom: '0.35em' },
            h3:   { marginTop: '0.7em', marginBottom: '0.3em' },
            pre:  { marginTop: '0.6em', marginBottom: '0.6em' },
            blockquote: { marginTop: '0.6em', marginBottom: '0.6em' },
          },
        },
        invert: {
          css: {
            lineHeight: '1.6',
          },
        },
      },
    },
  },
  plugins: [
    require('@tailwindcss/typography'),
  ],
}
