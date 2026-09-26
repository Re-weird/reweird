# UI/UX Design Notes

Living doc for layout/placement decisions. Rough sketches only (ASCII
wireframes) — not visual design, just placement and hierarchy. Pair with
`docs/project-hub-redesign-checklist.md` for the feature/build checklist.

---

## Dashboard (project hub)

GitHub-repo-list model. Flat list, no widgets, no charts, no per-project data
surfaced here.

```
┌─────────────────────────────────────────────────────────────────────┐
│ ReWeird           [ 🔍 Search projects...        ]   [+ New] [👤 ▾] │
├─────────────────────────────────────────────────────────────────────┤
│                                                                       │
│  My projects                                                         │
│  ─────────────────────────────────────────────────────────────────  │
│                                                                       │
│  ● Ultrasonic Parking Sensor          ESP32   🟢 Healthy   2h ago    │
│  ─────────────────────────────────────────────────────────────────  │
│  ● Servo Gate Controller              ESP32   🔴 Issue found  1d ago │
│  ─────────────────────────────────────────────────────────────────  │
│  ● LED Strip Rig                      ESP32   ⚪ Unconfirmed  5d ago │
│  ─────────────────────────────────────────────────────────────────  │
│  ● Weather Station v2                 ESP32   ⚫ No data yet 2w ago  │
│  ─────────────────────────────────────────────────────────────────  │
│                                                                       │
└─────────────────────────────────────────────────────────────────────┘
```

**Row anatomy** (left → right): project name (link, click → project home) ·
controller tag · status badge (color-coded verdict) · last-activity
timestamp (relative).

**Empty state** (no projects yet):

```
┌─────────────────────────────────────────────────────────────────────┐
│ ReWeird           [ 🔍 Search projects...        ]   [+ New] [👤 ▾] │
├─────────────────────────────────────────────────────────────────────┤
│                                                                       │
│                                                                       │
│                     No projects yet.                                 │
│                 [ + Create your first project ]                      │
│                                                                       │
│                                                                       │
└─────────────────────────────────────────────────────────────────────┘
```

**Explicitly excluded from this page:** live probe cards, signal charts,
computer-mode summary, git-sync status, PROBE/AI text — all of that is
per-project, not dashboard-level. See "why" in
`project-hub-redesign-checklist.md`.

**Shipped since this sketch:** name/description search, a controller filter,
and sort (last updated / name) in a bar above the list, plus a per-project
public/private toggle. A status filter was tried and removed; the rows no
longer show analysis-status labels.

---

## Login / Signup (placeholder — not sketched yet)

TODO once auth session-strategy decision lands in the checklist.

## Project page (placeholder — not sketched yet)

TODO — depends on the still-open "tabs vs separate routes" decision in the
checklist. Will map features #5-#13 into whichever shape we pick.
