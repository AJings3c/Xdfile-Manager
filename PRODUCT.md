# Product

## Register

product

## Users

Xdfile Manager is for terminal-first users who manage local and remote files for long sessions: developers, operators, and power users who want two panels, a command line, previews, and SSH/NetBox access in one TUI.

## Product Purpose

Xdfile Manager exists to be a dependable terminal file workbench. Success means users can browse, compare, copy, move, preview, run commands, and use F2 macros without leaving the terminal or losing context.

## Brand Personality

Focused, capable, terminal-native. The interface should feel dense but readable, direct rather than decorative, and confident about keyboard-heavy workflows while still supporting mouse actions.

## Anti-references

Do not turn the product into a GUI file manager. Do not copy superfile compatibility at the cost of Xdfile-specific PTY, NetBox, and F2 macro workflows. Avoid marketing-page composition, decorative chrome, large empty panels, and short-term promises for AI, plugins, cloud sync, or unsupported legacy config fields.

## Design Principles

- Keep the file operation workflow first: panels, terminal, status, and conflict prompts must stay predictable.
- Prefer compact, explicit UI copy that fits 80x24 terminals and remains comfortable at 120x32.
- Make dangerous operations clear before they write or delete anything.
- Preserve TUI familiarity: standard keys, visible state, stable focus, and no decorative motion.
- Treat remote operations as first-class but conservative, especially where undo is unavailable.

## Accessibility & Inclusion

The product should remain keyboard-first and usable without a mouse. Text must stay readable across the built-in Persona themes, status/error states must not rely on color alone, and the core layout must avoid overlap at 80x24, 120x32, and wider terminal sizes.
