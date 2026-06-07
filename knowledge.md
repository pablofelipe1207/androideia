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

