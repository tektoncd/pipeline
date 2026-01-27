# Tekton Pipelines Roadmap

**Project Board:** https://github.com/orgs/tektoncd/projects/16

## Purpose

The Tekton Pipelines Roadmap project board tracks the development roadmap for the `tektoncd/pipeline` component. It provides visibility into planned features, ongoing work, and completed items including:

- Feature requests and enhancements
- TEPs (Tekton Enhancement Proposals)
- Epics and their sub-issues
- Infrastructure improvements

See [The Tekton mission and vision](https://github.com/tektoncd/community/blob/main/roadmap.md#mission-and-vision)
for more information on our long-term goals.

## Status Columns

| Status | Meaning |
|--------|---------|
| **Todo** | Planned work, not yet started |
| **In Progress** | Actively being worked on |
| **Review** | Implementation complete, awaiting review |
| **Done** | Completed and closed |
| **Hold** | Paused or blocked |

## How Items Get Added

### Automatic Addition (Recommended)

Add the `area/roadmap` label to any open issue or PR in `tektoncd/pipeline`. The automation will:
1. Add the item to the project board
2. Set its status to **Todo**

Sub-issues of items already on the board are automatically added as well.

### Manual Addition

Maintainers can manually add items via the project board interface for items that don't have the `area/roadmap` label.

## How Items Move

Most status transitions happen automatically based on GitHub events:

```
┌─────────────────────────────────────────────────────────────────┐
│                        ENTRY POINTS                             │
├─────────────────────────────────────────────────────────────────┤
│  Label "area/roadmap" added       ───────────────► Todo         │
│  Sub-issue created                ───────────────► Todo         │
│  Manually added to project        ───────────────► Todo         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    AUTOMATED TRANSITIONS                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   Todo                                                          │
│     │                                                           │
│     │  PR linked to issue                                       │
│     │  Changes requested on PR                                  │
│     ▼                                                           │
│   In Progress                                                   │
│     │                                                           │
│     │  Issue closed                                             │
│     │  PR merged                                                │
│     ▼                                                           │
│   Done                                                          │
│     │                                                           │
│     │  Issue reopened ──────────────────────► In Progress       │
│     │                                                           │
│     │  Closed > 1 year with no updates                          │
│     ▼                                                           │
│   [Archived]                                                    │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│                     MANUAL TRANSITIONS                          │
├─────────────────────────────────────────────────────────────────┤
│   Any status ◄──────────────────────────────► Hold              │
│   In Progress ──────────────────────────────► Review            │
└─────────────────────────────────────────────────────────────────┘
```

## Best Practices

### For Contributors

| Action | How |
|--------|-----|
| **Get your feature on the roadmap** | Add `area/roadmap` label to your issue |
| **Signal you're working on something** | Link your PR to the issue — status moves to "In Progress" automatically |
| **Break down large features** | Use GitHub sub-issues; they're auto-added to the board |
| **Check what's planned** | Review the Todo column before proposing duplicate work |

### For Maintainers

| Action | How |
|--------|-----|
| **Prioritize roadmap items** | Use the board's drag-and-drop to reorder within columns |
| **Block work temporarily** | Move items to Hold manually; add a comment explaining why |
| **Signal review needed** | Move items to Review manually when implementation is complete |
| **Keep the board clean** | Items auto-archive after 1 year closed, but you can archive manually |
| **Track epic progress** | Use sub-issues — the board shows sub-issue completion progress |

### Things to Avoid

- **Don't manually set Done** — let the automation handle it when issues close or PRs merge
- **Don't remove `area/roadmap` label** to remove from board — the item stays; archive it instead if needed
- **Don't duplicate items** — search the board before adding new roadmap items
