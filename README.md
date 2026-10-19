# Renbun (錬文 - Sentence Alchemy)
A high-velocity, distributed Japanese literature miner and Discord bot built in Go.

### 🎯 **Project Overview & Motivation**
Renbun was engineered to automate the extraction of native Japanese sentences from the live web for immersion and spaced repetition (SRS) pipelines. Moving beyond sequential Python scraping, this engine demonstrates production-grade backend architecture by utilizing Go's M:N scheduler, concurrent worker pools, persistent WebSocket connections, and a modular command router to navigate and refine raw HTML into structured linguistic data.

### ⚡ **System Architecture**
`[Discord UI]` <-> `[WebSocket Gateway]` <-> `[Command Router]` <-> `[Worker Pools / SQLite / Gemini Oracle]`

* **The Frontend:** Headless command execution via the Discord API (WebSocket).
* **The Orchestrator:** A modular Go-based command router that safely isolates domain logic (Mining, Vaults, System) and manages concurrent worker pools to prevent resource exhaustion.
* **The Extraction:** DOM parsing utilizing `goquery` with automatic Shift_JIS/EUC-JP character encoding decoding and Furigana stripping.
* **The Grimoire Engine (DB):** A relational SQLite architecture managing robust CRUD operations, paginated caching, and persistent user vaults.
* **The Oracle (LLM):** Direct integration with Gemini 3.5 Flash Lite and 3.6 Flash for high-speed morphological parsing, contextual translation, and JLPT analysis.

### 🚀 **Core Commands**
**🔍 Scrying & Acquisition**
* `!mine <keyword>` – Triggers the distributed worker pool across `target.txt`, returning refined immersion sentences.
* `!save <indices> to <vault>` – Binds mined fragments into a persistent grimoire.
* `!add <vault> <sentence>` – Manually injects a raw sentence into a grimoire.

**📖 Grimoire Management**
* `!vault list` | `!vault create <name>` | `!vault delete <name>` – Manage your core database vaults.
* `!vault open <name> [page]` – Open and paginate through a grimoire's contents.
* `!search <keyword> [page]` – Execute a cross-vault global search.
* `!delete <index>` / `!clear <vault>` – Purge specific fragments or wipe entire grimoires.

**👁️ The Oracle (Inspection)**
* `!present <index/text>` – Rapidly reveals kana reading and English translation.
* `!dissect <index/text>` – Performs a deep morphological breakdown (JLPT level, nuance, grammar points, and vocabulary).

### 🛠️ **Development Roadmap**
* [x] Phase 0: Infrastructure & Authentication provisioning.
* [x] Phase 1: WebSocket connection & Discord Event Handlers.
* [x] Phase 2: Core DOM Extraction Logic & Shift_JIS translation.
* [x] Phase 3: Distributed Worker Pool Implementation.
* [x] Phase 4: Integration, Mathematical Filtering, and Rich UI.
* [x] Phase 5: SQLite Relational Database & Vault CRUD operations.
* [x] Phase 6: Gemini API Integration (Morphological Analysis).
* [x] Phase 7: Modular Command Router Refactoring.

### 🔮 **Future Expansion**
* **Lazy Persistence (Quota Armor):** Caching morphological dissection results locally in SQLite to completely eliminate redundant API calls and latency on previously analyzed sentences.
* **Tagging Metadata:** Implementing inline hashtag extraction during sentence binding to enable contextual categorization and targeted retrieval.
* **Anki/SRS Export Pipeline:** Generating localized `.csv` or `.apkg` packages directly from SQLite vaults for seamless integration into spaced repetition workflows.
* **Semantic Vector Search:** Upgrading the global search engine with embeddings to find contextually and semantically similar sentences beyond exact keyword matches.

### 🚀 **Quick Start**
Renbun is designed to be fully self-hosted. This ensures web scraping targets and API quotas remain strictly under your control.
#### Direct Execution
1. Navigate to the [Releases](../../releases) page and download the latest `renbun.exe`.
2. In the exact same directory as the executable, create a `.env` file containing your credentials:
    ```env
    DISCORD_TOKEN=your_discord_bot_token_here
    GEMINI_API_KEY=your_gemini_api_key_here
    ```
3. Run renbun.exe. The application will automatically generate the local renbun.db SQLite database and bring the bot online.

#### Build from Source
1. Clone the repository and navigate into the directory.
    ```
    git clone https://github.com/Nadif243/renbun.git
    cd renbun-engine
    ```
2. Create the .env file in the root directory with your Discord and Gemini credentials.
3. Build and execute the engine:
    ```
    go build -o renbun.exe ./cmd/bot
    ./renbun.exe
    ```
(Alternatively, execute directly in-memory via go run ./cmd/bot)
