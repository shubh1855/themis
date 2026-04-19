# Architecture Specification: Yash's Professional Portfolio

## 1. Tech Stack
- **Framework**: Next.js 14+ (App Router)
- **Styling**: Tailwind CSS
- **Language**: TypeScript
- **Icons**: Lucide React
- **Animations**: Framer Motion

## 2. Folder Structure
```text
src/
├── app/
│   ├── layout.tsx       # Root layout, providers, navbar, footer
│   ├── page.tsx         # Home page (Hero, About, Featured Projects)
│   ├── projects/
│   │   ├── page.tsx      # Projects listing
│   │   └── [slug]/page.tsx # Project detail page
│   └── globals.css       # Tailwind directives and custom variables
├── components/
│   ├── atoms/
│   │   ├── Button.tsx    # Standardized buttons
│   │   ├── Text.tsx      # Typography wrappers
│   │   ├── Badge.tsx     # Tech stack tags
│   │   └── Input.tsx     # Form fields
│   ├── molecules/
│   │   ├── ProjectCard.tsx # Image + Title + Short desc
│   │   ├── SkillBadge.tsx  # Icon + Label
│   │   ├── NavLink.tsx     # Animated navigation link
│   │   └── SocialLink.tsx  # Icon + URL
│   └── organisms/
│       ├── Navbar.tsx      # Top navigation with mobile menu
│       ├── HeroSection.tsx # Headline, Sub-headline, CTA
│       ├── ProjectGrid.tsx # Filterable list of ProjectCards
│       ├── SkillsSection.tsx # Grouped skills display
│       └── Footer.tsx      # Copyright, socials, contact
├── constants/
│   └── index.ts          # Centralized data for projects and skills
├── types/
│   └── index.ts          # TypeScript interfaces
└── lib/
    └── utils.ts         # Tailwind merge (clsx + tailwind-merge)
```

## 3. Design System (Premium Dark Mode)

### Color Palette
- **Background**: `bg-slate-950` (#020617) - Deep midnight blue/black
- **Surface**: `bg-slate-900` (#0f172a) - For cards and sections
- **Primary (Accent)**: `text-indigo-500` / `bg-indigo-600` (#6366f1) - Vibrant electric indigo
- **Secondary (Accent)**: `text-emerald-400` (#34d399) - For success/highlights
- **Text Primary**: `text-slate-100` (#f1f5f9) - Near white
- **Text Secondary**: `text-slate-400` (#94a3b8) - Muted grey
- **Border**: `border-slate-800` (#1e293b)

### Typography
- **Headings**: *Inter* or *Geist Sans* (Bold, tracking-tight)
- **Body**: *Inter* (Regular/Medium, leading-relaxed)
- **Mono (Code/Labels)**: *JetBrains Mono* or *Fira Code*

## 4. Data Structures

To ensure the portfolio is easy to update, all content is decoupled from the components.

### Project Interface
```typescript
interface Project {
  id: string;
  title: string;
  description: string;
  longDescription: string; // For detail page
  thumbnail: string;
  tags: string[];
  githubUrl?: string;
  liveUrl?: string;
  featured: boolean;
  period: string; // e.g., "Jan 2023 - Mar 2023"
}
```

### Skill Interface
```typescript
interface Skill {
  name: string;
  category: 'Frontend' | 'Backend' | 'DevOps' | 'Tools';
  level: 'Beginner' | 'Intermediate' | 'Advanced' | 'Expert';
  icon: React.ReactNode;
}
```

## 5. Key Architectural Decisions
- **Content-First**: Use a `constants/index.ts` file for all data. This allows a future transition to a CMS (Contentful/Sanity) without changing component logic.
- **Atomic Design**: Separating components into atoms, molecules, and organisms prevents the "mega-component" anti-pattern and ensures visual consistency.
- **Client-Side Interactivity**: Use `'use client'` only at the organism level or specific interaction components to maximize Next.js Server Component benefits for SEO and speed.