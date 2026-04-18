import type { Config } from 'tailwindcss';

const config: Config = {
  content: [
    './src/pages/**/*.{js,ts,jsx,tsx,mdx}',
    './src/components/**/*.{js,ts,jsx,tsx,mdx}',
    './src/app/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  theme: {
    extend: {
      colors: {
        background: '#020617',
        surface: '#0f172a',
        primary: {
          DEFAULT: '#6366f1',
          foreground: '#ffffff',
        },
        secondary: '#34d399',
        border: '#1e293b',
      },
    },
  },
  plugins: [],
};

export default config;