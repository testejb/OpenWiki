# Candidate Review Doc Contract

This contract applies to Feishu/Lark documents that contain both protocol lines:

```text
OPENWIKI_CANDIDATE_REVIEW_DOC v1
admission: ONLY_CHECKED_CANDIDATES
```

Such documents are candidate review documents, not ordinary sources.

## Mandatory behavior

- Treat as candidate review document, not ordinary source.
- Use lark-doc DocxXML so checkbox state can be inspected.
- Only title-checkbox checked candidate cards may be source material.
- Unchecked candidate cards are explicit user rejection.
- Do not summarize, merge, cite, quote, or use unchecked cards as background context.
- If no candidate cards checked, stop and write nothing.
- Checked candidate cards represent prior user approval. Do not ask for a second confirmation before ingesting them.

## Interaction override

- Checked cards replace the normal Step 3 confirmation prompt.
- For checked candidate cards, skip the Step 3 "Anything specific..." wait for user confirmation.
- Do not ask emphasize/de-emphasize or scope confirmation for checked cards.
- Use `target_wiki_area`, slug, category, and `proposed_content` from the card.
- If slug is missing, generate one using `slug-rules.md` and report the generated slug.
- If proposed content is missing, skip that candidate and report the skipped candidate.
- The final report may list processed candidates and any skip reasons.

## Candidate boundary

- A candidate starts at a title line or checkbox block in this format:

  ```text
  CAND-001｜slug｜title
  ```

- The candidate continues until the next candidate title line/checkbox block or a category title.
- Only the title checkbox controls admission.
- Ignore body checkboxes; they must not admit or reject a candidate.

## Fallback checkbox syntax

When DocxXML checkbox controls are unavailable but markdown-like title lines are present:

- `[x] CAND-001｜slug｜title` is accepted.
- `[ ] CAND-001｜slug｜title` is rejected.

Fallback syntax applies only to title lines. Body checkboxes are still ignored.

## Failure cases

- If the protocol is incomplete, stop and write nothing.
- If checkbox state cannot be determined, stop and write nothing.
- If a checked candidate lacks proposed content, skip it and report the skipped candidate.
- If a checked candidate lacks a slug, generate one using `slug-rules.md`.
- If a checked candidate lacks evidence, ingestion may continue, but mark the result as evidence-limited.
