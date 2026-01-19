---
phase: 05-analysis-intelligence
plan: 01
subsystem: analysis
tags: [bleve, bm25, cv, jd, matching, mcp]

# Dependency graph
requires:
  - phase: 03-cv-parsing
    provides: DocumentConverter interface for unified document handling
  - phase: 02-observability
    provides: Structured logging with slog/zerolog
provides:
  - Analysis engine with bleve BM25 scoring for CV/JD comparison
  - MCP tool for structured analysis (analyze_cv_jd)
  - Match percentage 0-100 based on content similarity
  - Skill coverage and gap analysis
affects: 06-testing, 07-polish (future phases can use analysis engine)

# Tech tracking
tech-stack:
  added:
    - github.com/blevesearch/bleve/v2 v2.5.7 (BM25 scoring)
    - github.com/blevesearch/bleve_index_api v1.2.11 (document interfaces)
  patterns:
    - BM25-based algorithmic matching (not LLM-based)
    - Structured JSON response format for MCP tools
    - Frontmatter stripping from storage documents

key-files:
  created:
    - internal/analysis/engine.go (339 lines) - Core bleve BM25 engine
    - internal/analysis/engine_test.go (387 lines) - 15 test scenarios
    - internal/mcp/analyze_tool.go (169 lines) - MCP tool implementation
    - internal/mcp/analyze_tool_test.go (344 lines) - 13 tool tests
  modified:
    - go.mod - Added bleve v2.5.7 and dependencies
    - go.sum - Updated with bleve checksums
    - internal/mcp/server.go - Registered analyze_cv_jd tool

key-decisions:
  - "Used bleve's BM25 scoring algorithm - no custom vector implementation"
  - "Strip YAML frontmatter from storage docs before analysis"
  - "Return structured JSON, not text prompts (separates algorithm from LLM)"

patterns-established:
  - "AnalysisEngine with Analyze(ctx, cv, jd) pattern - context-aware analysis"
  - "MCP tools with frontmatter-aware document reading"
  - "Test-driven development with 15 engine tests + 13 tool tests"

# Metrics
duration: 12min
completed: 2026-01-17
---

# Phase 05-analysis-intelligence Plan 01 Summary

**BM25-based CV/JD analysis engine with bleve v2.5.7 and structured MCP tool for match scoring**

## Performance

- **Duration:** 12 min
- **Started:** 2026-01-17T19:38:00Z
- **Completed:** 2026-01-17T19:50:00Z
- **Tasks:** 5
- **Files modified:** 6

## Accomplishments

- Created analysis package with bleve BM25 scoring engine
- Implemented structured analysis returning match percentage 0-100%
- Added MCP tool for CV/JD comparison with JSON output
- Registered analyze_cv_jd tool in MCP server
- Full test coverage for engine and tool

## Task Commits

1. **Task 1: Create analysis package** - `363f8aa` (feat)
2. **Task 2: Install bleve dependency** - `78b0447` (chore)
3. **Task 3: Implement analysis engine** - `363f8aa` (feat, included in Task 1)
4. **Task 4: Create MCP analyze tool** - `e69e5e7` (feat)
5. **Task 5: Register tool in server** - `b49b06a` (feat)

**Plan metadata:** `b49b06a` (docs: complete 05-01 plan)

## Files Created/Modified

- `internal/analysis/engine.go` - Bleve-based analysis engine with BM25 scoring
- `internal/analysis/engine_test.go` - 15 comprehensive test scenarios
- `internal/mcp/analyze_tool.go` - MCP tool for structured analysis
- `internal/mcp/analyze_tool_test.go` - Tool validation tests
- `go.mod` - Added bleve v2.5.7 and dependencies
- `go.sum` - Updated checksums for bleve packages
- `internal/mcp/server.go` - Registered analyze_cv_jd tool

## Decisions Made

- Used bleve v2.5.7 BM25 scoring (no custom vector implementation)
- Strip YAML frontmatter from storage documents before analysis
- Return structured JSON (not text prompts) to separate algorithm from LLM
- Created dedicated analysis package separate from mcp package
- Match percentage calculated algorithmically from term frequencies

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed bleve v2 API compatibility**

- **Found during:** Task 3 (Analysis engine implementation)
- **Issue:** Existing code used bleve v1 API (bleve.Document, TermVectors) which doesn't exist in v2.5.7
- **Fix:** Updated to use bleve v2 API:
  - `bleve.IndexMapping` → `mapping.IndexMapping`
  - `bleve.Document` → `index.Document` (from bleve_index_api)
  - `TermVectors` → `AnalyzedTokenFrequencies()` via search queries
  - Added proper imports for `mapping` and `index` packages
- **Files modified:** internal/analysis/engine.go
- **Verification:** All 15 analysis tests pass, all 13 tool tests pass
- **Committed in:** `363f8aa` (Task 1)

**2. [Rule 2 - Missing Critical] Frontmatter stripping**

- **Found during:** Task 4 (MCP tool implementation)
- **Issue:** Storage documents contain YAML frontmatter (id, original_filename, ingested_at, type), causing false analysis results
- **Fix:** Added `stripFrontmatter()` regex to remove `---\n...\n---` before analysis
- **Files modified:** internal/mcp/analyze_tool.go
- **Verification:** Identical documents now correctly show 100% match
- **Committed in:** `e69e5e7` (Task 4)

**3. [Rule 1 - Bug] Unused import removal**

- **Found during:** Task 5 (Server registration)
- **Issue:** server.go imported analysis package but didn't use it directly (tools use it indirectly)
- **Fix:** Removed unused import to prevent compilation warnings
- **Files modified:** internal/mcp/server.go
- **Verification:** `go build ./...` succeeds without warnings
- **Committed in:** `b49b06a` (Task 5)

---

**Total deviations:** 3 auto-fixed (1 blocking, 1 missing critical, 1 bug)
**Impact on plan:** All auto-fixes necessary for correct operation. No scope creep.

## Issues Encountered

- **Bleve v2 API mismatch:** Had to adapt existing analysis code to current bleve v2.5.7 API (vs v1 which was referenced in plan)
- **Storage document format:** Documents stored with YAML frontmatter required extraction before analysis

## Next Phase Readiness

- ✅ Analysis engine ready for integration with other tools
- ✅ MCP tool registered and discoverable
- ✅ All tests passing (analysis: 15/15, mcp: 13/13)
- ✅ Build succeeds without errors
- ✅ BM25 scoring produces reasonable match percentages
- ⚠️ Ready for 06-testing phase (integration tests, end-to-end tests)
- ⚠️ Ready for 07-polish phase (docs, edge cases, performance optimization)

---
*Phase: 05-analysis-intelligence*
*Completed: 2026-01-17*
