---
applyTo: "**/*.md"
---

# Documentation Writing Requirements

## Language
- Use British English spelling and phrasing.
- Keep tone clear, direct, and practical.

## Keep Docs Stable
- Do not duplicate information that can be reliably discovered from the codebase.
- Avoid directory trees, exhaustive file lists, and restating concrete implementation details.
- Avoid documenting fast-changing details such as exact internal call flows unless required for understanding behaviour.

## Prefer High-Value Content
- Explain purpose, constraints, decisions, and trade-offs.
- Document user-visible behaviour, workflows, and operational guidance.
- Capture invariants and guarantees that readers should rely on.
- Include examples only when they clarify usage or expected outcomes.

## Maintainability Rules
- Treat code as the source of truth for structure and low-level mechanics.
- Keep documentation concise and easy to update.
- When details are likely to change, describe principles rather than brittle specifics.