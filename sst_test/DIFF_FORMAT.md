# SST Diff File Format Specification

> This document formalizes the binary format used by `SstWriteDiff` / `SstReadDiff` for future debugging and troubleshooting.

---

## 1. Overall File Structure

```
Diff File
├── Header          (imported/referenced graph level differences)
├── Dictionaries    (node-level differences for all graphs)
│   ├── Current Graph
│   ├── Imported Graphs
│   └── Referenced Graphs
└── Content         (node-level triple differences)
```

The diff file does **not** contain a magic header. It starts directly with `importedCount`.

---

## 2. Header (Graph-Level Differences)

### 2.1 Format

```
importedCount     (uvarint)
referencedCount   (uvarint)

// Imported graph entries (total: importedCount)
for each imported graph:
  flag            (uvarint)  // 0=Same, 1=Removed, 2=Added
  [if Same]
    span          (uvarint)  // number of consecutive Same graphs
  [if Removed or Added]
    graphIRI      (string)

// Referenced graph entries (total: referencedCount)
for each referenced graph:
  same as above
```

### 2.2 Semantics of `Same` Span

`Same, span=N` means **N consecutive graphs that are identical in both from and to graph lists**.

Example:
- from referenced: `[sso, rdf, rdfs, ns3]`
- to referenced:   `[sso, rdf, ns3]`
- diff header:
  - `Same, span=2`   → sso + rdf
  - `Removed`        → rdfs
  - `Same, span=1`   → ns3

**Note**: The entry count in the diff header is not necessarily equal to the from or to graph count. It is the merged diff entry count.

---

## 3. Dictionaries (Node-Level Differences)

Dictionaries are written in the following order:

1. **Current Graph**
2. **Imported Graphs** (in header order)
3. **Referenced Graphs** (in header order)

### 3.1 Current Graph Dictionary

```
iriDeleted        (uvarint)  // number of deleted IRI nodes
iriAdded          (uvarint)  // number of added IRI nodes
blankDeleted      (uvarint)  // number of deleted blank nodes
blankAdded        (uvarint)  // number of added blank nodes

// IRI node entries (total: computeDiffNodeCount(from.IRI, to.IRI))
for each entry:
  flag            (uvarint)  // 0=Same, 1=Removed, 2=Added, 3=TripleModified
  [if Same]
    span          (uvarint)
  [if Removed or Added]
    fragment      (string)
    triplexDelta  (varint)   // triple count delta relative to from/to
  [if TripleModified]
    triplexDelta  (varint)

// Blank node entries (same as above, total: computeDiffNodeCount(from.Blank, to.Blank))
```

### 3.2 Imported / Referenced Graph Dictionary

```
deleted           (uvarint)  // number of deleted nodes
added             (uvarint)  // number of added nodes

// Node entries (total: computeDiffNodeCount(from.Nodes, to.Nodes))
for each entry:
  same format as Current Graph
```

### 3.3 Node Entry Types

| Flag | Type | Data | Description |
|------|------|------|-------------|
| 0 | Same | `span` | span consecutive unchanged nodes |
| 1 | Removed | `fragment`, `triplexDelta` | node only exists in from |
| 2 | Added | `fragment`, `triplexDelta` | node only exists in to |
| 3 | TripleModified | `triplexDelta` | node exists but triple count changed |

**Key Rule**: `computeDiffNodeCount` is **not** simply `deleted + added`. It is the **total number of merged sorted entries** (Same + TripleModified + Removed + Added).

---

## 4. Content (Triple-Level Differences)

Content is written sequentially for each combined node in order. The format for each node's content is:

```
// 1. Identical span (optional)
[if identicalContentSpan > 0]
  identicalSpanEncoded  (uvarint) = span << 1

// 2. Non-literal triples header
removedShifted  (uvarint) = (removedCount << 1) | diffModifiedFlag(0b01)
added           (uvarint)

// 3. Non-literal triples (only if removed+added > 0)
for each same triple:
  sameCountEncoded  (uvarint) = count << 2
for each removed triple:
  predIndexShifted  (uvarint) = (predIndex << 2) | 0b01
  objIndex          (uvarint)
for each added triple:
  predIndexShifted  (uvarint) = (predIndex << 2) | 0b11
  objIndex          (uvarint)

// 4. Literal triples header
literalRemovedShifted  (uvarint) = (removedCount << 1) | diffModifiedFlag(0b01)
literalAdded           (uvarint)

// 5. Literal triples (only if removed+added > 0)
for each same literal triple:
  sameCountEncoded  (uvarint) = count << 2
for each removed literal triple:
  predIndexShifted  (uvarint) = (predIndex << 2) | 0b01
  kind              (uvarint)  // 0=string, 1=langString, 2=typed, ...
  value             (depends on kind)
for each added literal triple:
  predIndexShifted  (uvarint) = (predIndex << 2) | 0b11
  kind              (uvarint)
  value             (depends on kind)
```

### 4.1 Encoding Constants

```go
const (
    diffIdenticalOrModifiedOffset = 1
    diffModifiedFlag              = 0b01
    diffSameRemovedOrAddedOffset  = 2
)
```

### 4.2 Identical Span Encoding

**Writer**: `span << diffIdenticalOrModifiedOffset` (i.e., `span << 1`)

**Reader**: After reading a value, if `value & diffModifiedFlag == 0`, it is an identical span:
```go
identicalNodeCount = (value >> diffIdenticalOrModifiedOffset) + 1
```

The `+1` compensates for the `> 1` decrement logic in `readCountDeltaNodeNodes`, so the net number of identical entries processed equals `span`.

### 4.3 Modified Header with Zero Changes

When `removed == 0 && added == 0` (node content unchanged), the writer writes only the header `(0 << 1) | 1 = 1` followed by `added = 0`. **No same triple counts are written** — the reader reads directly from the base stream.

> This behavior was fixed to prevent diff stream misalignment. Previously, same triple counts were also written when `removed+added == 0`, but the reader did not consume them, causing the diff reader to drift and later panic.

### 4.3 Modified Header Encoding

Both non-literal and literal section headers use:
```go
removedShifted = (removedCount << diffIdenticalOrModifiedOffset) | diffModifiedFlag
```

When the reader detects `value & diffModifiedFlag != 0`, it knows this is a modified header:
```go
removed = value >> diffIdenticalOrModifiedOffset
added   = readUint(r)
```

### 4.4 Triple Encoding

| Low 2 bits | Meaning | Data |
|-----------|---------|------|
| `0b00` | Same | `count = value >> 2` |
| `0b01` | Removed | `pred = value >> 2`, then `obj` or `kind+value` |
| `0b11` | Added | `pred = value >> 2`, then `obj` or `kind+value` |

`predIndex` is the **combined node index** (see Section 5).

---

## 5. Combined Node Index

All node references in content (e.g., `predIndex`) use the **combined node index**, not a per-graph local index.

### 5.1 Construction Order

The combined node list is built in the following order, matching the diff dictionary parsing order exactly:

```
1. Current Graph IRI nodes
   - Same nodes (in from order)
   - TripleModified nodes (in from order)
   - Removed nodes (in from order)
   - Added nodes (in to order)
2. Current Graph Blank nodes (same as above)
3. Imported Graphs (in header order, each graph: Same → Removed → Added)
4. Referenced Graphs (same as above)
5. Added Imported Graphs (in to order)
6. Added Referenced Graphs (in to order)
```

### 5.2 Example

For diff from `[sso, rdf, rdfs, ns3]` → to `[sso, rdf, ns3]`:

| Combined Index | Graph | Fragment | Entry Type |
|---------------|-------|----------|------------|
| 0 | current | `` | Same |
| 1 | current | `caaad355-...` | TripleModified |
| 2 | imported | `f07d725b-...` | Same |
| 3 | referenced | `PartSpecification` | Same |
| 4 | referenced | `type` | Same |
| 5 | referenced | `comment` | **Removed** |
| 6 | referenced | `1dfd12ca-...` | Same |

Therefore, `pred=5` → `rdfs:comment`.

---

## 6. Writer / Reader Interaction Flow

### 6.1 Writer (`sstwritediff.go`)

1. `diffWriteContext` builds from/to node translations
2. `diffWriteHeader`: Compares from/to imported/referenced graph lists, outputs diff entries
3. `diffWriteDictionary`: Compares nodes per graph, outputs Same/Removed/Added/TripleModified entries
4. `diffWriteContent`: For each combined node, compares triples, outputs identical/modified/removed/added

### 6.2 Reader (`sstfiledeltaspanread.go`)

1. `readCountDeltaNodeNodes`: Reads non-literal section header, handles identical span
2. `readCountDeltaNodeLiterals`: Reads literal section header, handles identical span
3. When reading triples, looks up predicate/object using the combined node index

---

## 7. Known Issues and Debugging Tips

### 7.1 Panic in `readCountDeltaNodeLiterals` — `removed+added == 0` Same Counts Mismatch

- **Symptom**: `TestDebugBlankNodeDiff` / `TestWriteDiffModifyOneOfBlankNodes` panics at `sstfiledeltaspanread.go:563` with "unrecognized diff entry"
- **Root Cause**: When `removed == 0 && added == 0` in `diffWriteNodeTriples`, the writer was writing same triple counts to the diff stream, but the reader skipped them and read directly from base. This caused the diff reader to drift, so subsequent reads hit wrong data (e.g., an identical span encoding instead of a literal header).
- **Fix**: In `diffWriteNodeTriples`, skip writing `d.nodeTripleBuf` when `removed+added == 0`.

### 7.2 `computeDiffNodeCount` Is Not `deleted + added`

- `len(combinedNodes) = Same + TripleModified + Removed + Added`
- When parsing diff dictionaries, you must use `computeDiffNodeCount`, not `deleted + added` directly.

### 7.3 Debugging Recommendations

1. Use `TestParseDiffBinaryFormat` to parse the diff file and verify the combined node map is correct.
2. If panicking in `readCountDeltaNodeLiterals`, check whether the diff reader has drifted — usually caused by writer/reader mismatch on same triple counts when `removed+added == 0`.
3. If pred index looks wrong, check whether the combined node map construction order matches the writer's order.
