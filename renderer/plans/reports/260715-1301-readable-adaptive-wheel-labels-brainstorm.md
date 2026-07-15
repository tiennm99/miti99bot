---
title: "Readable Adaptive Wheel Labels Brainstorm"
date: 2026-07-15
status: approved
mode: markdown
tags: [brainstorm, renderer, typography, ux]
---

# Readable Adaptive Wheel Labels Brainstorm

## Summary

Approved direction: treat 14px as the preferred readability threshold. Attempt one-line fit first; when it falls below 14px and slice height permits, wrap deterministically into two or three balanced lines. For dense wheels that cannot safely fit multiple lines, preserve the complete label at the existing smaller fallback size.

## Problem-First Analysis

### 1. Solution-Jumping Diagnosis

The request is not fundamentally “add more newlines.” The observed failure is that the current wrapping trigger models technical fit at the 8px hard floor, not practical readability.

### 2. Underlying Problem

Users cannot comfortably read medium-length option labels even when the slice has enough vertical space to display them across multiple lines.

### 3. Assumption Challenges

| Assumption | Risk if wrong | Validation |
|---|---|---|
| 8px is a sufficient wrapping threshold | Text technically fits but remains unreadable | Probe and render 9–13px labels |
| Every long label can wrap | Dense slices overlap adjacent labels | Derive maximum lines from slice arc height |
| Character count predicts readability | Wide/narrow glyphs and Vietnamese marks vary | Continue using estimated rendered width |
| Full text must always remain visible | Dense wheels may force unreadably small type | Preserve full text as explicit dense fallback |

### 4. Problem Statement

- Users: GIF API consumers rendering descriptive wheel entries.
- Context: supported 384px, 480px, and 512px wheels with 2–32 options.
- Struggle: a phrase such as `Chiều nay uống CraneTea` remains one line at 9px on a 512px wheel.
- Cause: wrapping only activates when the one-line fit reaches exactly 8px.
- Consequence: available slice height goes unused and labels are difficult to read.
- Success: medium-length labels wrap at readable sizes whenever geometry permits, without overlap or API changes.

### 5. Alternative Framings

1. Threshold problem: the 8px trigger is too late; use a preferred readable size.
2. Geometry problem: allocate lines from both radial width and tangential slice height.
3. Information-density problem: dense wheels cannot simultaneously preserve full text, large type, and non-overlap.

### 6. Evidence Status

Medium. The user supplied a concrete production-style Vietnamese label, and direct layout probes reproduce the issue across all option counts at 512px. Existing tests cover only the 8px boundary, not the unreadable 9–13px range.

### 7. Validation Plan

- Add exact layout cases for `Chiều nay uống CraneTea` at every supported size.
- Cover low, medium, and dense option counts.
- Assert preferred font size when wrapping is possible and bounded height when it is not.
- Render a Vietnamese fixture and visually check boldness, clipping, line balance, and adjacent-slice separation.
- Kill the design if 14px wrapping causes overlap at supported counts where the helper reports multiple lines.

### 8. Stakeholder Message

We will improve readability without changing the API: labels below the preferred 14px one-line size will wrap when the slice has room. Dense wheels will continue showing complete text at a smaller size because enlarging it would overlap neighboring slices.

## Evaluated Approaches

### A. Fixed Readability Threshold — Approved

Trigger wrapping when one-line fit is below 14px and at least two lines fit safely.

- Pros: predictable UX; directly fixes 9–13px blind spot; easy to test; keeps current geometry model.
- Cons: 14px is a product decision; dense wheels still require small text.
- Example at 512px: `Chiều / nay uống / CraneTea` for low counts; two balanced lines when only two fit.

### B. Relative Threshold

Wrap when one-line fit falls below a percentage of the computed base font size.

- Pros: scales with wheel and option density.
- Cons: harder to explain; may still allow unreadably small absolute sizes; more boundary churn.

### C. Always Wrap Multiword Labels

Prefer two or three lines for every multiword entry.

- Pros: maximizes font size for phrases.
- Cons: over-wraps short labels, increases visual noise, and repeats the regression already caught for ordinary names.

## Approved Design

1. Keep deterministic, Unicode-safe word/grapheme splitting.
2. Compute one-line size against padded content width.
3. Define a preferred readable threshold of 14px, separate from the existing 8px hard minimum.
4. If one-line size is at least 14px, preserve one line.
5. If it is below 14px and `maxLines >= 2`, choose the most balanced two/three-line layout and size from its longest line.
6. If geometry allows only one line, preserve the complete label using the existing 8px minimum behavior.
7. Keep bold weight, radial track footprint, rotations, API schemas, themes, and winner metadata unchanged.

## Exact Requirements

- Expected output: updated Remotion label layout behavior plus focused tests and visual render evidence.
- Acceptance: the example phrase wraps at 512px for 2–16 options when geometry permits; resulting wrapped text targets at least 14px; ordinary short labels remain one line; 24–32 option layouts remain bounded and preserve full text; Vietnamese graphemes remain intact.
- Out of scope: truncation, tooltips, larger API sizes, curved SVG text, API/schema changes, theme redesign.
- Constraints: Node.js 24, JavaScript + JSDoc, existing React/Remotion/CSS patterns, deterministic output.
- Touchpoints: `src/remotion/wheel-label-layout.js`, `test/wheel-label-layout.test.js`, and renderer fixture/smoke evidence; `WheelComposition.jsx` only if new layout metadata is required.

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| 14px bold estimate still clips | Measure against padded content width and visually render fixtures |
| Dense wheels remain small | Document as intentional geometry fallback; keep full text |
| New threshold re-wraps short names | Regression-test 384px smoke labels and 512px short labels |
| Vietnamese split corruption | Preserve `Intl.Segmenter` grapheme fallback and add accented-text tests |

## Success Metrics

- Example phrase no longer renders as a 9px single line where two/three lines fit.
- No supported-size regression for short one-line labels.
- No label box exceeds its computed tangential height.
- Lint, typecheck, tests, `render:smoke`, and a Vietnamese visual fixture pass.
- Public API contracts remain unchanged.

## Unresolved Questions

None. Dense-wheel fallback and 14px threshold are approved.

## Next Step

Create a tests-first implementation plan because this changes existing label layout behavior with established regression coverage.
