# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/Nadif243/renbun/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/Nadif243/renbun/releases/tag/v1.0.0
