# Workbench 2

The first question is: **What can I open, and what needs attention?**

This is a local launcher and project workspace. Projects, their real launch commands,
HTTP readiness, process identity, and recent use supply the content. The existing shared
registry remains the source of truth. Skills and process-level diagnostics are secondary views.

The frontend skill dealt `framed-sheet`, seed 1709902781. A low-chroma green-grey frame,
white working sheet, 228 px sidebar, 48 px top bar, 24 px canvas padding, 16 px gaps,
quiet edges, and restrained status colours preserve Workbench's identity. System sans
type preserves the existing brand; monospace is reserved for ports, commands and logs.
No new font dependency or external images are required.

Breakscale's public site (https://breakscale.tech/, inspected 2026-10-26) describes a
system canvas with an editable topology. The inspiration here is an arrangeable workspace
and an adjacent inspector. Workbench uses its own project cards, list view and launcher
workflow. There are no simulated connections or infrastructure diagrams.

The board reorders real project cards. Dragging changes personal order, never service state.
Keyboard arrow controls provide the same operation. Personal order, favourites, recent opens,
theme and view choice persist in this browser. Shared project records remain in registry.json.

The primary actions are Open for a ready local site, Start for a stopped project, and
Inspect or Logs when readiness or identity needs attention. Starting presents the exact
server-owned launch command before the user's explicit click. Stop retains its separate
human confirmation and fresh PID identity check. Unknown process ownership never becomes
permission to stop or claim a service.

Readiness is an HTTP response at the registered local URL, not a full application health
certification. Discovery is bounded, skips secrets/build/dependency trees and symlinks,
and proposes projects for review. It never starts or imports a project automatically.
