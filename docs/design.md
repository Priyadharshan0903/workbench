# Workbench 3 dashboard

The project's first question is **What can I open, and what needs attention?**

The app-screen design skill dealt True-black flat, seed 439850117, on 2026-10-26.
The choice suits a personal developer workspace: one neutral ground, surfaces separated by
lightness, restrained status colour and an inverted primary action. The product's face is the
workbench itself: the W mark, real launch commands, folder paths and local port ownership.
The screen's signature element is the project card's command preview next to its verified state.

## Real objects and jobs

- Objects are registered projects, owned listening processes, detached runs and installed skills.
- A project has a name, folder, launch command, URL, reserved ports and lifecycle.
- A service has a PID, working folder, endpoints and an evidence-based project association.
- Readiness combines an owned listener with a responding local HTTP URL.
- The common job is opening an existing project; the next is starting a stopped one.
- Failed starts point to logs; unverified ownership points to Diagnostics.
- The person uses this dashboard on their own computer, often beside their editor.
- Small catalogs are real: zero or one user project must work as well as a busy catalog.
- Workbench's engine is installation infrastructure, shown in the header and Diagnostics.
- Favourites, recent opens, ordering and theme are browser preferences, not catalog changes.

## Layout and theme

The sidebar and 64px header share a neutral #0c0c0c ground. Flat #171717 panels supply the next
lightness step; #222 and #2a2a2a are nested/control surfaces. A muted green is reserved for ready
services. Tabs and navigation use neutral ink, and the main Find projects action is inverted.
Inter 4.1 is bundled under its OFL license and served from the executable, with no external font
request. Mono is reserved for commands, paths, ports and terminal output.

The grid arithmetic is 1440 = 224px sidebar + 60px gutters + 1156px content. Cards share a 14px
gap, with a lone card spanning its row. One project becomes a compact full-width record. A new
catalog gets a first-run panel with real actions instead of an oversized card for the engine.
The old duplicate status summary is accessible without repeating its figures visually.
Tabs count projects in the current search, lifecycle and navigation scope.

The existing version-2 preference key remains. Theme version 3 switches earlier preferences to
dark once, keeping favourites, order and recent opens; explicit subsequent light/dark selections
persist. The initial HTML is dark. Light mode remains available as a neutral alternative.

At 1000px the sidebar folds away. At phone widths the toolbar wraps, cards stack and icon hit
areas grow to 44px. Dialogs retain native focus trapping, Escape handling and inert backgrounds.

## Screen questions and behaviour

- Projects: what is ready, and which project should I start or inspect?
- First run: how do I add a real project folder?
- Diagnostics: which process owns this port, and can its identity be verified?
- Agent skills: which skills are present, and where did they come from?
- Project details/logs: which command ran, what happened and what should I change?

Drag and keyboard ordering only change personal arrangement. Starting presents the exact
server-owned launch plan for confirmation. Stop retains human confirmation and a fresh PID
identity check. Discovery proposes records for review; it never imports or starts automatically.

## Verification

Frontend QA is manual after retirement of the standalone UI test harness. Check interactions,
escaped content, error recovery, first-run actions, filtered totals, engine separation and
preference migration.
Go tests cover the new embedded font route and license inclusion in clean distribution archives.
The design source lint has no failures. Its warnings are intentional: monospace for code/IDs,
a shadow only behind the opened mobile navigation, and em dashes for missing-value placeholders.

Chrome exits before rendering, including on an elevated retry, so real screenshot inspection at
1440, 768 and 390 is unverified in this environment. No sample projects or invented metrics have
been added to the real catalog; Go test fixtures use isolated temporary catalogs.

## Launch setup

A registered web project with a command but missing its local address or port offers
**Set up & start**. The existing settings dialog collects those details; saving opens the
server-owned launch plan for explicit confirmation. Existing styles and modal focus handling
are reused. A failed validation or cancelled review never starts a command. Manually check this
path, including matching local URL/port checks and viewing logs after Start.
