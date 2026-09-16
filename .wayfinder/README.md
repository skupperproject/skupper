# Wayfinder: Proxy Support for Non-Kube Sites

This directory contains the wayfinding map for extending HTTP proxy support to local system sites (podman, docker, systemd).

## Structure

- **`map.md`**: The canonical map - shows the destination, decisions made so far, and fog of war
- **`tickets/`**: Individual decision tickets, one per file

## Working the Map

### Current Frontier (Unblocked Tickets)

To see which tickets are ready to work:

```bash
grep -L "blocks: \[" tickets/*.md | grep "status: open"
```

Or manually check which tickets have `blocks: []` (empty) and `status: open`.

### Claiming a Ticket

When you start working a ticket:

1. Edit the ticket file, change `status: open` to `status: claimed`
2. Add your name to frontmatter: `assignee: <your-name>`

### Resolving a Ticket

When a decision is reached:

1. Add a `## Resolution` section to the ticket with the answer
2. Change `status: claimed` to `status: closed`
3. Update `map.md`:
   - Add one line to "Decisions so far" section linking to the ticket
   - Clear any related items from "Not yet specified" if they're now decided
   - Add any new fog that the decision revealed to "Not yet specified"

### Ticket Types

- **research** (AFK): Reading code, docs, or external resources to surface facts
- **grilling** (HITL): Conversation to resolve design decisions - call the grilling skill
- **prototype** (HITL): Create concrete artifacts to react to (examples, diagrams, stubs)

## Quick Reference

**Map**: `.wayfinder/map.md`

**Tickets**: Numbered `001-`, `002-`, etc. in `tickets/` directory

**Blocking**: Edit frontmatter `blocks: [003, 004]` to indicate dependencies
