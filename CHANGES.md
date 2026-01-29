# 0.7.1

## 1. JSON Path Array Indexing (`context.go`)

**What:** Adds array index syntax to variable resolution.

```
{{steps.api.tracks[0].name}}  ← now works
```

**How:** Regex parses `key[index]` segments, accesses map then array. `jsonPathLookup` now delegates to `jsonPathLookupRaw` (no more duplicated logic).

---

## 2. File Discovery (`tui.go`)

**What:** Changed from "git-tracked files" → "non-gitignored files".

**Impact:**
- Untracked `.yapi` files now discovered (previously invisible until committed)
- Properly recurses into submodules

**How:** Replaced git index scan with `filepath.Walk` + gitignore pattern matching.

---

## Breaking Changes

None expected. New files may appear in selection that weren't visible before.
