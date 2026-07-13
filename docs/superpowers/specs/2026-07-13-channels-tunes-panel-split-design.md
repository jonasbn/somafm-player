# Design: Split Channels/Tunes panel + discoverability fixes

Date: 2026-07-13
Source: `better_ux-ui.md` (repo root TODO notes)

## Problem

The TUI currently has two boxes: Now Playing, and a single List box whose
content switches between four modes (Channels, Bookmarked Channels,
Bookmarked Tunes, History) via a breadcrumb tab strip (`c`/`f`/`s`/`H` keys).
This conflates two conceptually different things — "what can I play" and
"what did I play" — into one box the user has to keep flipping between.
Additionally, the `j`/`k` movement keys, while present in the footer, aren't
easily discoverable.

## Goals

1. Split the single tabbed List box into two always-visible boxes:
   Channels and Tunes.
2. Channels box: default to showing bookmarked channels if any exist, else
   show all channels; `a` toggles between All and Bookmarked.
3. Tunes box: default to History (what's actually been played — history
   can't be "replayed" so it's the more useful default); Bookmarked Tunes
   is a secondary view reachable via existing key.
4. Make `j`/`k` movement more discoverable without changing its behavior.

## Non-goals

- No change to playback logic, audio pipeline, or history/bookmark
  persistence formats.
- No general responsive-layout system — just enough width plumbing to make
  two side-by-side boxes render cleanly.
- No new bookmark or history features beyond what's needed to support the
  split.

## Data model & state changes

Replace the single `viewMode` enum (`viewChannels`, `viewBookmarkedChannels`,
`viewBookmarkedTunes`, `viewHistory`) with two independent states that are
visible at the same time:

- `channelsFilter`: `filterBookmarked` | `filterAll` — toggled by `a`
- `tunesMode`: `tunesHistory` | `tunesBookmarked` — set explicitly by `H`
  (history) / `s` (bookmarked tunes); not a toggle pair, kept as direct jumps

**Startup default for `channelsFilter`**, computed once in `New()`:
`filterBookmarked` if `len(cfg.BookmarkedChannels) > 0`, else `filterAll`.

`focusArea` gains a third value: `focusNowPlaying`, `focusChannels`,
`focusTunes` (was `focusNowPlaying`/`focusList`). `Tab` cycles through all
three in that order.

**Selection state**: since both list boxes are on-screen and independently
scrollable simultaneously, the single `selected int` field splits into
`channelSelected int` and `tuneSelected int`, each reset to `0` when its own
filter/mode changes (mirrors today's `switchMode` reset behavior).

**Downstream effects**:
- `currentListLen()` and `selectedChannel()` split into per-box variants.
- `handleBookmarkKey()` (`bookmark_actions.go`) branches on `focus` (Channels
  vs Tunes) then on that box's own filter/mode, instead of the old single
  `mode` switch.
- `enter`-to-play remains Channels-box-only (unchanged — history/bookmarked
  tunes were never playable).

## Layout & rendering

Screen structure (top to bottom): `renderNowPlaying` (full width) →
Channels box | Tunes box (side by side) → footer.

**New minimal state**: add `width int` to `Model`, populated via a new
`case tea.WindowSizeMsg` in `Update()` (no such handling exists today).
Falls back to a default of 80 columns before the first resize message
arrives.

**Sizing**: each list box gets `.Width((m.width - overhead) / 2)` via
lipgloss, where `overhead` accounts for border/padding on both boxes, so the
two boxes render at a balanced, even split rather than each sizing to its
own content.

**Rendering**: `renderChannelsBox` and `renderTunesBox` (replacing
`renderList`), composed with
`lipgloss.JoinHorizontal(lipgloss.Top, channelsBox, tunesBox)`.

- Channels box header: `Channels ▸ [Bookmarked]` or `Channels ▸ [All]`
  (bracket convention matches today's `listHeader`). Body: same per-row
  rendering as today's `viewChannels`/`viewBookmarkedChannels` (★ marker +
  title + genre for All; plain titles for Bookmarked).
- Tunes box header: `Tunes ▸ [History]` or `Tunes ▸ [Bookmarked]`. Body:
  same row formatting as today's `viewHistory`
  (`Title — Artist (Channel) @ HH:MM:SS`) and `viewBookmarkedTunes`
  (`Title — Artist (Channel)`).
- Both boxes keep the existing focus-highlight border convention
  (`borderStyle(t, focused)`).

## Keybindings & discoverability

Changes:
- `a` — new, toggles Channels box filter (`filterBookmarked` ↔ `filterAll`)
- `c`, `f` — removed (redundant with `a`)
- `H`, `s` — unchanged, jump Tunes box to History / Bookmarked Tunes
- `Tab` — now cycles `focusNowPlaying → focusChannels → focusTunes →
  focusNowPlaying`
- `j`/`k` (and `up`/`down`) — unchanged behavior, act on whichever of the
  two list boxes currently has focus
- `b`, `enter`, `+`/`-`, `m`, `t`, `r`, `q` — unchanged

**Discoverability fix for j/k**: the root issue is that the footer crams
every binding into one dense line, not that the keys don't work. Two
changes:
1. Reorder/group the footer logically (movement, boxes, playback, app) so
   `j/k move` isn't buried mid-line.
2. Add a short per-box hint into each box's header row, e.g.
   `Channels ▸ [Bookmarked]  (a) all  (j/k) move`.

Footer becomes roughly:
`[Theme: X]  tab focus · j/k move · enter play · b bookmark · a all/bookmarked · H/s tunes · +/- vol · m mute · t theme · r retry · q quit`

## Testing

- Unit tests for the new per-box selection/filter logic (`channelsFilter`
  default computation, `a` toggle, `channelSelected`/`tuneSelected` reset on
  filter/mode change) — extend existing `internal/ui` test patterns.
- Unit tests for `handleBookmarkKey()`'s updated focus/filter branching.
- Rendering/layout is not unit-testable in a meaningful way here (per
  project CLAUDE.md: no TTY in sandboxed agent environments) — manual
  verification via `go run .` in a real terminal is required for visual
  confirmation, consistent with existing project practice.
