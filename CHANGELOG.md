# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `!source open <category> [page]` lists the targets inside a category, numbered with the index `!source remove` expects. The category can be given by name or by its number from `!source list`.

### Changed
- `!source list` now shows categories in a fixed alphabetical order, so the numbers no longer reshuffle between calls.
- `!source remove` accepts a category number as well as its name.

### Removed
- Unused `loadTargets` helper left over from the `target.txt` era.

### Fixed
- Deleting a grimoire that still contains fragments no longer fails with a foreign key error; `!vault delete` now removes the grimoire together with all of its fragments, leaving no orphaned data behind.
- `!mine <keyword> in <category>` now matches category names regardless of letter case, like `!source` (`in it` finds `IT`, and `in all` mines every source).

## [1.2.0] - 2026-10-21

### Added
- **Exact-Match Command Aliases:** Introduced fast shortcuts (`!v`, `!m`, `!d`, `!p`) with dynamic `SplitN` argument parsing.
- **Oracle Status Reactions:** Added instant `⏳`, `✅`, and `⚠️` emoji feedback to track LLM execution states.
- **Native UI Buttons**: Upgraded pagination from just direct command to native Discord Buttons (`⬅️`, `✖️`, `➡️`) featuring dynamic boundary disabling.
- **Transient UI Menus:** Implemented a global 90-second expiration fade and supercede wiping system for all non-essential bot prompts to maintain chat cleanliness.
- **Conversational Text Navigation:** Users can now seamlessly type `next`, `prev`, `cancel`, or a specific page number to navigate active menus without clicking.
- **Grimoire Dropdowns**: Upgraded `!v list` to utilize Discord String Select Menus that seamlessly morph into paginated viewers.
- **Prompt Purging:** Added a state interceptor that instantly deletes user command inputs during menu navigation (requires "Manage Messages" server permission).

### Changed
- **Oracle-Miner Bridge:** Updated `resolveTargetText` to prioritize the transient mining cache, enabling a direct `!mine` -> `!dissect`/`!present` pipelines.

### Fixed
- Reverted vault sorting to `ASC` to ensure dropdown index matches the `!v open <index>` sequence.
- Resolved Go compiler `rune` type mismatch by utilizing `db.GetVaultNames` slice extraction.
- Fixed SQL linter warnings by adding final `rows.Err()` validation checks in database loops.
- Fixed 404 (Unknown Message) error logging caused by manual or superseded message deletions during the transient fade cycle.

## [1.1.0] - 2026-10-20

### Added
- Dynamic source engine: mining targets now live in the SQLite `sources` table
  instead of `target.txt`, with a seeder that loads baseline sources on first boot.
- `!source list | add | remove` commands to manage sources and categories from Discord.
- `!mine <keyword> in <category>` to mine from a single source category.
- Error logging for scraper worker failures.

### Changed
- Oracle now uses the `google.golang.org/genai` SDK, since the previous Gemini SDK is
  deprecated. This is an internal change; commands behave the same.
- Dissect now sets an explicit thinking level and no longer overrides temperature,
  which Gemini 3 models are tuned to leave at the default.
- The `!help` menu documents the source commands and the `in <category>` syntax.

### Fixed
- `!dissect` no longer hangs or wastes daily API quota on timeouts: the timeout is now
  actually applied to the request, and model replies are read through the SDK's text
  accessor instead of the first raw part, which could contain the model's reasoning.
- Scraper requests send browser-like headers, so Fandom wikis no longer block them.

## [1.0.0] - 2026-10-12

### Added
- First official release.
- Modular command router for System, Mining, Vault, and Oracle operations.
- Command router and categorized !help menu.
- !search <keyword>: cross-vault global search.
- !mine <keyword>: concurrent worker pool scraping with Shift_JIS/EUC-JP decoding, furigana removal, and sentence filtering.
- Persistent SQLite vaults: !vault list | create | delete | open, !save, !add, !delete, !clear, with paginated viewing.
- Gemini API integration for lightweight translation (`!present`) and deep morphological breakdown (`!dissect`).
- SQLite-backed grimoire engine for saving, searching, and managing mined sentences.
- Multi-threaded worker pool for high-speed sentence extraction.

[Unreleased]: https://github.com/Nadif243/renbun/compare/v1.2.0...HEAD
[1.2.0]: https://github.com/Nadif243/renbun/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/Nadif243/renbun/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/Nadif243/renbun/releases/tag/v1.0.0
