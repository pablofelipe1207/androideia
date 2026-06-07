# Project Knowledge

Exported from androideai-core brain

## Use Hilt for DI

**Type:** decision  
**Status:** promoted  
**Created:** 2026-06-05 12:13:36  

**Tags:** di,hilt  

We decided to use Hilt for dependency injection

---

## Never overwrite existing code

**Type:** rule  
**Status:** promoted  
**Created:** 2026-06-07  

**Tags:** safety,code-integrity,preserve  

When adding new features, NEVER overwrite or delete existing code in files like MainActivity, AndroidManifest, or navigation graphs. Always create new files for new functionality and add to existing files without removing working code. Only overwrite when the user explicitly asks for it and after confirming with confirm_plan.

---

## Design consistency — existing vs new features

**Type:** rule  
**Status:** promoted  
**Created:** 2026-06-07  

**Tags:** design,consistency,ui,ux  

- **Existing features**: Their visual design, UI/UX, themes, colors, typography, spacing, navigation and component styles are IMMUTABLE. Do NOT change them unless user explicitly requests a redesign.
- **New features/screens**: MUST follow the design system of existing screens. Before creating a new Composable Screen, inspect existing screens with `semantic_locate type=composable` or `android_scaffold role=composable action=check` and replicate:
  - MaterialTheme usage (colors, typography, shapes)
  - Spacing/Dimens system
  - Component patterns (buttons, inputs, cards, navigation bars)
  - State patterns (Loading, Error, Empty, Success)
  - Animations/transitions
  - Layout structure (Scaffold, Column, LazyColumn, etc.)
---

