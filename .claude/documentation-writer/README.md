# Documentation Writer Knowledge Base

This directory contains documentation standards and templates for the pgEdge
Postgres MCP Server project.

## Purpose

This knowledge base provides:

- Complete documentation style guide
- Templates for common document types
- Examples of well-written documentation
- Formatting rules and conventions

## Documents

### [style-guide.md](style-guide.md)

**AUTHORITATIVE** - Complete style guide derived from CLAUDE.md:

- Writing style rules
- Document structure requirements
- List formatting
- Code snippet formatting
- Link and reference rules
- README requirements

### [templates.md](templates.md)

Ready-to-use templates:

- README template
- API documentation template
- Feature documentation template
- Changelog entry format

### [examples.md](examples.md)

Links to well-written documentation in the project:

- Good README examples
- Good API documentation
- Good inline documentation

## Quick Reference

### Critical Rules

The documentation writer must follow these rules:

- Wrap all markdown files at 79 characters.
- Use active voice throughout.
- Write sentences between 7 and 20 words that are grammatically complete.
- Leave a blank line before every list, including sub-lists.
- Do not use emojis unless explicitly requested.
- Use four-space indentation in code blocks.

### Document Location

| Document Type | Location |
|---------------|----------|
| Sub-project docs | `/docs/<subproject>/` |
| Sub-project README | `/<subproject>/README.md` |
| Top-level README | `/README.md` |
| Changelog | `/docs/changelog.md` |

### File Naming

- Use **lowercase** for all files in `/docs/`
- Use **hyphens** for multi-word names: `api-reference.md`
- Each sub-project docs has an `index.md` entry point

## Document Updates

This knowledge base is the source of truth for documentation standards.
Update these documents when:

- Style guide changes
- New templates needed
- New patterns established

Last Updated: 2026-01-09
