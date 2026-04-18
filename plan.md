# Implementation Plan: Yash's Professional Portfolio

## Project Summary
Build a high-performance, premium dark-mode portfolio website for Yash using Next.js 14, Tailwind CSS, and TypeScript. The site focuses on a polished aesthetic, easy content updates via centralized data constants, and an Atomic Design component architecture.

## Tech Stack
- **Framework**: Next.js 14 (App Router)
- **Styling**: Tailwind CSS
- **Type Safety**: TypeScript
- **Animations**: Framer Motion
- **Icons**: Lucide React

## Folder Structure
Refer to `docs/architecture.md` for the detailed annotated tree.

## Risk Register
| Risk | Likelihood | Mitigation |
| :--- | :--- | :--- |
| Content update friction | Low | Centralized `constants/index.ts` for all data |
| Performance issues (Animations) | Medium | Use Framer Motion `layout` props and avoid over-animating |
| Responsiveness gaps | Medium | Mobile-first development approach with Tailwind breakpoints |

## Milestones

### M1: Foundation & Scaffolding
- **Owner**: Hephaestus
- **Tasks**:
    - Initialize Next.js project with TypeScript and Tailwind.
    - Implement `docs/architecture.md` folder structure.
    - Configure `tailwind.config.ts` with the premium dark-mode palette.
    - Set up `lib/utils.ts` (clsx + tailwind-merge).
- **Definition of Done**: Project runs locally, folder structure exists, and a basic page displays the primary brand colors.

### M2: Data Layer & Atomic Components
- **Owner**: Hephaestus
- **Tasks**:
    - Define TypeScript interfaces in `types/index.ts`.
    - Populate `constants/index.ts` with mock project and skill data.
    - Build **Atoms**: Button, Text, Badge, Input.
- **Definition of Done**: All data types defined; Atoms are visually consistent and reusable.

### M3: Molecules & Organisms (Core Sections)
- **Owner**: Hephaestus
- **Tasks**:
    - Build **Molecules**: ProjectCard, SkillBadge, NavLink.
    - Build **Organisms**: Navbar, HeroSection, SkillsSection.
    - Integrate Framer Motion for subtle entry animations.
- **Definition of Done**: Home page layout is complete and visually matching the architecture spec.

### M4: Project Gallery & Dynamic Routing
- **Owner**: Hephaestus
- **Tasks**:
    - Build ProjectGrid organism with filtering logic.
    - Implement dynamic route `/projects/[slug]` for detailed project views.
    - Connect dynamic pages to the `constants/index.ts` data.
- **Definition of Done**: Users can navigate from home to a specific project detail page.

### M5: Polish, SEO & Final Review
- **Owner**: Ares / Hermes
- **Tasks**:
    - Add Meta tags, OpenGraph images, and Favicon.
    - Implement responsive checks across mobile/tablet/desktop.
    - Final accessibility (a11y) audit.
- **Definition of Done**: Site is fully responsive, SEO optimized, and passes accessibility checks.

## Open Decisions
- Deployment target (Vercel is assumed).
- Contact form implementation (EmailJS vs. Formspree vs. Custom API).

## Deferred Scope
- Full Blog system (deferred to v2, can be added via MDX later).
- Admin dashboard for content updates (deferred in favor of `constants/index.ts`).