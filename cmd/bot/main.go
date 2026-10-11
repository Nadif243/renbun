package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"rennbunengine/internal/db"
	"rennbunengine/internal/oracle"
	"rennbunengine/internal/worker"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

type CacheEntry struct {
	Payload   []db.ExtractedSentence
	ExpiresAt time.Time
}

var userState sync.Map

// mineCache temporarily holds the latest mined sentences for each user.
var mineCache sync.Map // Maps m.Author.ID -> []db.ExtractedSentence

type VaultSession struct {
	VaultName string
	Sentences []string
}

// vaultCache remembers the last 10 sentences a user viewed via '!vault open'
var vaultCache sync.Map // Maps m.Author.ID -> []string (raw japanese text)

func main() {
	// 1. Load the secret token from the .env file
	err := godotenv.Load(".env")
	if err != nil {
		log.Println("Warning: No .env file found.")
	}

	// 2. Ignite the Crucible
	db.InitDB("./renbun.db")
	defer db.DB.Close()

	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		log.Fatal("CRITICAL: DISCORD_TOKEN is missing!")
	}

	// Ignite the Oracle
	geminiKey := os.Getenv("GEMINI_API_KEY")
	if geminiKey == "" {
		log.Fatal("Critical: GEMINI_API_KEY is missing from the void (.env)")
	}
	if err := oracle.InitOracle(geminiKey); err != nil {
		log.Fatal(err)
	}

	// 3. Initialize the Discord Session
	// The Discord API requires the exact prefix "Bot " before the token.
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatal("Error creating Discord session: ", err)
	}

	// 4. Register the Event Handler
	// This tells the Go runtime: "Every time a message is sent in the Discord server,
	// execute the 'messageCreate' function."
	dg.AddHandler(messageCreate)

	// 5. Declare Gateway Intents
	// This matches the "Message Content Intent" toggled in the Developer Portal.
	// Without this, Discord will censor the text of the messages sent to the bot.
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentMessageContent

	// 6. Open the WebSocket connection
	err = dg.Open()
	if err != nil {
		log.Fatal("Error opening connection: ", err)
	}

	// Set the Alchemy Custom Status
	err = dg.UpdateStatusComplex(discordgo.UpdateStatusData{
		Activities: []*discordgo.Activity{
			{
				Type:  discordgo.ActivityTypeCustom,
				Name:  "Custom Status",        // Discord API requires this field to be populated
				State: "錬金術駆動の没入型エンジンで言葉を錬成中", //Refining text through an alchemy-driven immersion engine
			},
		},
	})
	if err != nil {
		fmt.Println("Warning: Could not set custom status:", err)
	}

	// 7. Keep the Server Alive
	fmt.Println("Renbun Engine (錬文) is online. Press CTRL-C to exit.")

	// Create a channel to listen for OS-level interrupt signals (like Ctrl+C).
	// The main Goroutine completely freezes at `<-sc`, keeping the bot alive indefinitely.
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// 8. Clean Shutdown
	fmt.Println("\nShutting down Renbun Engine safely...")
	dg.Close()
}

// Helper function to read URLs from our text file
func loadTargets(filename string) []string {
	file, err := os.Open(filename)
	if err != nil {
		return nil
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && strings.HasPrefix(line, "http") {
			urls = append(urls, line)
		}
	}
	return urls
}

// This function is the callback for the event listener.
// discordgo automatically spawns a NEW Goroutine for this function every single time a message is received.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if m.Content == "!ping" {
		s.ChannelMessageSend(m.ChannelID, "Pong! 錬文 engine is listening.")
		return
	}

	// !mine [keyword] command
	if strings.HasPrefix(m.Content, "!mine ") {
		// Extract the URL from the message
		keyword := strings.TrimSpace(strings.TrimPrefix(m.Content, "!mine "))

		// 1. Load the target list
		urls := loadTargets("target.txt")
		if len(urls) == 0 {
			s.ChannelMessageSend(m.ChannelID, "⚠️ `target.txt` is empty or missing.")
			return
		}

		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🔍 **Mining %d sources** for the word: `%s`...", len(urls), keyword))

		// 2. Ignite the Worker Pool (using 5 concurrent workers)
		results := worker.RunPool(urls, 5)

		// 3. The Goldilocks Filter
		var validSentences []db.ExtractedSentence
		for _, res := range results {
			if res.Err != nil {
				continue // Silently ignore dead websites
			}
			for _, sentence := range res.Sentences {
				// Must contain the exact keyword
				if strings.Contains(sentence, keyword) {
					runeCount := len([]rune(sentence))
					// Must be longer than 15 chars (destroys headers/dates)
					// Must be shorter than 60 chars (destroys run-on paragraphs)
					if runeCount >= 15 && runeCount <= 60 {
						// Capture the text AND the URL
						validSentences = append(validSentences, db.ExtractedSentence{
							Keyword: keyword,
							RawText: sentence,
							Source:  res.URL, // Ensure 'URL' matches the field name in your worker result struct
						})
					}
				}
			}
		}

		// 4. Handle No Results
		if len(validSentences) == 0 {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("⚠️ No high-quality sentences found for `%s` matching the length criteria.", keyword))
			return
		}

		// 5. Shuffle the array to ensure fresh examples every search attempt
		rand.Shuffle(len(validSentences), func(i, j int) {
			validSentences[i], validSentences[j] = validSentences[j], validSentences[i]
		})

		// Limit output to the top 3
		limit := 3
		if len(validSentences) < limit {
			limit = len(validSentences)
		}

		// 6. Format and Ship the Output as a Rich Embed
		embed := &discordgo.MessageEmbed{
			Title:       fmt.Sprintf("錬文 Engine: %s", keyword),
			Description: fmt.Sprintf("Scanned **%d** sources.\nFound **%d** high-quality sentences.", len(urls), len(validSentences)),
			// Set the color to a sleek gold/amber (Hex: #F5A623)
			Color:  0xF5A623,
			Fields: []*discordgo.MessageEmbedField{},
		}

		// Prepare a slice to hold exactly what we show the user, so indices match perfectly
		var cachePayload []db.ExtractedSentence

		for i := 0; i < limit; i++ {
			extracted := validSentences[i]
			cachePayload = append(cachePayload, extracted)

			// Replace the target word with an inline code block so it has a distinct background
			highlighted := strings.ReplaceAll(extracted.RawText, keyword, fmt.Sprintf("`%s`", keyword))

			// Optional: We can now hyper-link the source directly in the embed using Discord's markdown
			valueText := fmt.Sprintf("%s\n*[Source Link](%s)*", highlighted, extracted.Source)

			// Append each sentence as its own distinct block in the UI
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("Result %d", i+1),
				Value:  valueText,
				Inline: false,
			})
		}

		// Seed the Transmutation Cache
		// We store ONLY the exact 3 (or fewer) sentences we displayed to the user.
		// If they type !save 1, it will perfectly match cachePayload[0].
		mineCache.Store(m.Author.ID, CacheEntry{
			Payload:   cachePayload,
			ExpiresAt: time.Now().Add(15 * time.Minute), // Cache rots after 15 minutes
		})

		// Using SendEmbed instead of Send
		s.ChannelMessageSendEmbed(m.ChannelID, embed)
	}

	// The Binding Command
	if m.Content == "!bind" {
		response, err := db.BindUser(m.Author.ID)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "⚠️ Alchemical failure during binding: "+err.Error())
			return
		}
		s.ChannelMessageSend(m.ChannelID, "🩸 "+response)
		return
	}

	// --- STATE INTERCEPTOR ---
	// If the user is in the middle of a process, intercept their message before checking commands.
	if state, ok := userState.Load(m.Author.ID); ok {
		input := strings.TrimSpace(m.Content)

		// Global cancel catch
		if input == "cancel" {
			userState.Delete(m.Author.ID)
			s.ChannelMessageSend(m.ChannelID, "Action aborted.")
			return
		}

		stateStr := state.(string)

		// State: Creating a Vault
		if stateStr == "awaiting_vault_create_name" {
			response, err := db.CreateVault(m.Author.ID, input)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ Failed to structure the vault: "+err.Error())
			} else {
				s.ChannelMessageSend(m.ChannelID, "📖 "+response)
			}
			userState.Delete(m.Author.ID)
			return
		}

		// State: Opening a Vault
		if stateStr == "awaiting_vault_open_name" {
			vaultName, err := db.ResolveVaultInput(m.Author.ID, input)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ "+err.Error())
				userState.Delete(m.Author.ID)
				return
			}

			sentences, err := db.OpenVault(m.Author.ID, vaultName)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ Failed to open vault: "+err.Error())
				userState.Delete(m.Author.ID)
				return
			}

			embed := &discordgo.MessageEmbed{
				Title: "📖 Vault Contents: 『 " + vaultName + " 』",
				Color: 0xC6D8F0,
			}
			if len(sentences) == 0 {
				embed.Description = "*This vault is currently empty.*"
			} else {
				var formatted []string
				for i, sent := range sentences {
					formatted = append(formatted, fmt.Sprintf("**%d.** %s", i+1, sent))
				}
				embed.Description = strings.Join(formatted, "\n\n")
			}

			s.ChannelMessageSendEmbed(m.ChannelID, embed)
			userState.Delete(m.Author.ID)
			return
		}

		// State: Deleting a Vault
		if stateStr == "awaiting_vault_delete_name" {
			vaultName, err := db.ResolveVaultInput(m.Author.ID, input)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ "+err.Error())
				userState.Delete(m.Author.ID)
				return
			}

			if vaultName == "Primary Grimoire" {
				s.ChannelMessageSend(m.ChannelID, "⚠️ The Primary Grimoire cannot be destroyed.")
				userState.Delete(m.Author.ID)
				return
			}

			response, err := db.DeleteVault(m.Author.ID, vaultName)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ Failed to incinerate vault: "+err.Error())
			} else {
				s.ChannelMessageSend(m.ChannelID, response)
			}

			userState.Delete(m.Author.ID)
			return
		}

		// State: Saving fragments (Missing Vault Target)
		if strings.HasPrefix(stateStr, "awaiting_save_vault|") {
			// Extract the indices they wanted to save
			indicesStr := strings.TrimPrefix(stateStr, "awaiting_save_vault|")
			indices := strings.Split(indicesStr, ",")

			// Resolve the vault they just typed in chat
			vaultName, err := db.ResolveVaultInput(m.Author.ID, input)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ "+err.Error())
				userState.Delete(m.Author.ID)
				return
			}

			// Pull from cache
			cachedData, ok := mineCache.Load(m.Author.ID)
			if !ok {
				s.ChannelMessageSend(m.ChannelID, "⚠️ Cache expired. Mine again.")
				userState.Delete(m.Author.ID)
				return
			}

			entry := cachedData.(CacheEntry)
			if time.Now().After(entry.ExpiresAt) {
				mineCache.Delete(m.Author.ID)
				s.ChannelMessageSend(m.ChannelID, "⚠️ Your mined fragments have decayed. Please run `!mine` again.")
				userState.Delete(m.Author.ID)
				return
			}

			extractedList := entry.Payload

			var selectedSentences []db.ExtractedSentence
			for _, idxStr := range indices {
				idx, _ := strconv.Atoi(idxStr) // Already validated in the initial command
				selectedSentences = append(selectedSentences, extractedList[idx-1])
			}

			// Execute Save
			count, err := db.SaveToVault(m.Author.ID, vaultName, selectedSentences)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ Failed to bind fragments: "+err.Error())
			} else if count == 0 {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🛡️ 『 %s 』: All selected fragments already exist here.", vaultName))
			} else {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🩸 Successfully bound **%d** new fragment(s) to 『 %s 』.", count, vaultName))
			}

			userState.Delete(m.Author.ID)
			return
		}
	}

	// --- THE VAULT ROUTER ---
	if strings.HasPrefix(m.Content, "!vault") {
		// Verify binding status FIRST to protect all subcommands
		var isBound bool
		err := db.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE discord_id = ?)", m.Author.ID).Scan(&isBound)
		if err != nil || !isBound {
			s.ChannelMessageSend(m.ChannelID, "⚠️ You are not bound to the crucible. Run `!bind` first.")
			return
		}

		// Split the command into arguments (e.g., "!vault", "create", "Slang")
		args := strings.Fields(m.Content)

		// If they just typed exactly "!vault"
		if len(args) == 1 {
			embed := &discordgo.MessageEmbed{
				Title: "📖 The Vault Grimoire",
				Color: 0xC6D8F0, // Powder Blue
				Description: "`!vault create <name>` - Forge a new collection\n" +
					"`!vault list` - View your active vaults\n" +
					"`!vault open <name or index>` - Inspect a vault's contents\n" +
					"`!vault delete <name or index>` - Incinerate a collection",
			}
			s.ChannelMessageSendEmbed(m.ChannelID, embed)
			return
		}

		// Subcommand branching
		subCommand := args[1]

		switch subCommand {
		case "create":
			// If they typed "!vault create" with no name
			if len(args) == 2 {
				// Put them in the state map to wait for their next message
				userState.Store(m.Author.ID, "awaiting_vault_create_name")
				s.ChannelMessageSend(m.ChannelID, "Enter the name for your new vault (or type `cancel`):")
				return
			}
			// If they typed "!vault create MyVault"
			vaultName := strings.Join(args[2:], " ") // Joins multi-word names
			response, err := db.CreateVault(m.Author.ID, vaultName)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ Error: "+err.Error())
				return
			}
			s.ChannelMessageSend(m.ChannelID, "📖 "+response)

		case "list":
			listData, err := db.ListVaults(m.Author.ID)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ Error reading the void: "+err.Error())
				return
			}
			embed := &discordgo.MessageEmbed{
				Title:       "🔍 Your Bound Grimoires",
				Color:       0xC6D8F0,
				Description: listData,
			}
			s.ChannelMessageSendEmbed(m.ChannelID, embed)

		case "open":
			if len(args) == 2 {
				// Fetch their list of vaults
				listData, err := db.ListVaults(m.Author.ID)
				if err != nil {
					s.ChannelMessageSend(m.ChannelID, "⚠️ Error reading the void: "+err.Error())
					return
				}

				userState.Store(m.Author.ID, "awaiting_vault_open_name")

				embed := &discordgo.MessageEmbed{
					Title:       "📖 Select a Grimoire to Open",
					Color:       0xC6D8F0,
					Description: listData + "\n\n*Reply with the name or index number (or type `cancel`).*",
				}
				s.ChannelMessageSendEmbed(m.ChannelID, embed)
				return
			}

			// The Smart Parser (Handles Vault Name/Index + Optional Page Number)
			input := strings.Join(args[2:], " ")
			parts := strings.Fields(input)
			page := 1
			vaultTarget := input

			vaultName, err := db.ResolveVaultInput(m.Author.ID, input)
			if err != nil && len(parts) > 1 {
				// If exact match fails, check if the last word is a page number
				lastPart := parts[len(parts)-1]
				if p, errConv := strconv.Atoi(lastPart); errConv == nil {
					page = p
					vaultTarget = strings.TrimSpace(strings.TrimSuffix(input, lastPart))
					vaultName, err = db.ResolveVaultInput(m.Author.ID, vaultTarget)
				}
			}

			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ "+err.Error())
				return
			}

			// Fetch the contents
			sentences, err := db.OpenVault(m.Author.ID, vaultName)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ Failed to open vault: "+err.Error())
				return
			}
			// SEED THE VAULT CACHE
			vaultCache.Store(m.Author.ID, VaultSession{
				VaultName: vaultName,
				Sentences: sentences,
			})

			if len(sentences) == 0 {
				embed := &discordgo.MessageEmbed{
					Title:       fmt.Sprintf("📖 Vault Contents: 『 %s 』", vaultName),
					Description: "*This vault is currently empty.*",
					Color:       0xC6D8F0,
				}
				s.ChannelMessageSendEmbed(m.ChannelID, embed)
				return
			}

			// --- Pagination Math ---
			perPage := 10 // Show 10 fragments per page
			maxPage := int(math.Ceil(float64(len(sentences)) / float64(perPage)))

			// Snap bounds
			if page < 1 {
				page = 1
			}
			if page > maxPage {
				page = maxPage // If they type page 99, just show the last page
			}

			start := (page - 1) * perPage
			end := start + perPage
			if end > len(sentences) {
				end = len(sentences)
			}

			displaySlice := sentences[start:end]

			// --- Build the Embed ---
			var descBuilder strings.Builder
			for i, text := range displaySlice {
				actualIndex := start + i + 1 // Ensure numbers match their global cache position (e.g., 11, 12, 13...)
				descBuilder.WriteString(fmt.Sprintf("**%d.** %s\n\n", actualIndex, text))
			}

			embed := &discordgo.MessageEmbed{
				Title:       fmt.Sprintf("📖 Vault Contents: 『 %s 』", vaultName),
				Description: descBuilder.String(),
				Color:       0x8B008B, // Dark Magenta
				Footer: &discordgo.MessageEmbedFooter{
					Text: fmt.Sprintf("Page %d of %d • Total Fragments: %d", page, maxPage, len(sentences)),
				},
			}

			s.ChannelMessageSendEmbed(m.ChannelID, embed)
			return

		case "delete":
			if len(args) == 2 {
				listData, err := db.ListVaults(m.Author.ID)
				if err != nil {
					s.ChannelMessageSend(m.ChannelID, "⚠️ Error reading the void: "+err.Error())
					return
				}

				userState.Store(m.Author.ID, "awaiting_vault_delete_name")

				embed := &discordgo.MessageEmbed{
					Title:       "🔥 Select a Grimoire to Incinerate",
					Color:       0xC6D8F0,
					Description: listData + "\n\n*Reply with the name or index number (or type `cancel`).*",
				}
				s.ChannelMessageSendEmbed(m.ChannelID, embed)
				return
			}

			// 1. Resolve the name/index
			input := strings.Join(args[2:], " ")
			vaultName, err := db.ResolveVaultInput(m.Author.ID, input)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ "+err.Error())
				return
			}

			if vaultName == "Primary Grimoire" {
				s.ChannelMessageSend(m.ChannelID, "⚠️ The Primary Grimoire cannot be destroyed.")
				return
			}

			response, err := db.DeleteVault(m.Author.ID, vaultName)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ Failed to incinerate vault: "+err.Error())
				return
			}
			s.ChannelMessageSend(m.ChannelID, response)

		default:
			s.ChannelMessageSend(m.ChannelID, "⚠️ Unknown vault command. Type `!vault` for the manual.")
		}
		return
	}

	// --- 3. THE SAVE ROUTER ---
	if strings.HasPrefix(m.Content, "!save") {
		// 1. Verify they actually mined something recently
		cachedData, ok := mineCache.Load(m.Author.ID)
		if !ok {
			s.ChannelMessageSend(m.ChannelID, "⚠️ You have no active transmutations in the cache. Run `!mine <word>` first.")
			return
		}

		entry := cachedData.(CacheEntry)
		if time.Now().After(entry.ExpiresAt) {
			mineCache.Delete(m.Author.ID) // Purge the rotted cache
			s.ChannelMessageSend(m.ChannelID, "⚠️ Your mined fragments have decayed. Please run `!mine` again.")
			return
		}

		extractedList := entry.Payload

		// 2. Parse the syntax
		content := strings.TrimSpace(strings.TrimPrefix(m.Content, "!save"))
		if content == "" {
			s.ChannelMessageSend(m.ChannelID, "⚠️ Specify which fragments to bind (e.g., `!save 1 2` or `!save 1 to 1`).")
			return
		}

		parts := strings.SplitN(content, " to ", 2)
		indicesStr := strings.Fields(parts[0]) // e.g., ["1", "3"]

		// Validate indices
		var selectedSentences []db.ExtractedSentence
		for _, idxStr := range indicesStr {
			idx, err := strconv.Atoi(idxStr)
			if err != nil || idx < 1 || idx > len(extractedList) {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("⚠️ Fragment `%s` does not exist in the current cache.", idxStr))
				return
			}
			selectedSentences = append(selectedSentences, extractedList[idx-1])
		}

		// 3. If they omitted the "to <vault>" target
		if len(parts) == 1 {
			listData, err := db.ListVaults(m.Author.ID)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ Error reading the void: "+err.Error())
				return
			}

			// We pass the indices safely via the state string (e.g., "awaiting_save_vault|1,3")
			statePayload := "awaiting_save_vault|" + strings.Join(indicesStr, ",")
			userState.Store(m.Author.ID, statePayload)

			embed := &discordgo.MessageEmbed{
				Title:       "📖 Select a Destination Grimoire",
				Color:       0xC6D8F0,
				Description: listData + "\n\n*Reply with the name or index number (or type `cancel`).*",
			}
			s.ChannelMessageSendEmbed(m.ChannelID, embed)
			return
		}

		// 4. If they provided the full command (e.g., !save 1 2 to 1, Slang)
		vaultTargets := strings.Split(parts[1], ",")
		var successLogs []string

		for _, rawTarget := range vaultTargets {
			target := strings.TrimSpace(rawTarget)
			if target == "" {
				continue
			}

			vaultName, err := db.ResolveVaultInput(m.Author.ID, target)
			if err != nil {
				successLogs = append(successLogs, fmt.Sprintf("⚠️ `%s`: %s", target, err.Error()))
				continue
			}

			count, err := db.SaveToVault(m.Author.ID, vaultName, selectedSentences)
			if err != nil {
				successLogs = append(successLogs, fmt.Sprintf("⚠️ `%s`: Failed to bind - %s", vaultName, err.Error()))
			} else if count == 0 {
				successLogs = append(successLogs, fmt.Sprintf("🛡️ 『 %s 』: All selected fragments already exist here.", vaultName))
			} else {
				successLogs = append(successLogs, fmt.Sprintf("🩸 『 %s 』: Bound **%d** new fragment(s).", vaultName, count))
			}
		}

		s.ChannelMessageSend(m.ChannelID, strings.Join(successLogs, "\n"))
		return
	}

	// --- 4. THE ORACLE ROUTER ---

	// Helper function to resolve the target text (Index vs Raw Text)
	resolveTargetText := func(input string) (string, error) {
		input = strings.TrimSpace(input)
		if input == "" {
			return "", fmt.Errorf("provide a vault index (e.g., `1`) or raw Japanese text.")
		}
		// If they typed a number, pull from vaultCache
		if idx, err := strconv.Atoi(input); err == nil {
			cachedData, ok := vaultCache.Load(m.Author.ID)
			if !ok {
				return "", fmt.Errorf("no open grimoire found. Run `!vault open <name>` first.")
			}

			session := cachedData.(VaultSession)
			sentences := session.Sentences

			if idx < 1 || idx > len(sentences) {
				return "", fmt.Errorf("index %d does not exist in your open grimoire.", idx)
			}
			return sentences[idx-1], nil
		}
		// If it's not a number, assume they typed raw Japanese text
		return input, nil
	}

	// !present Command
	if strings.HasPrefix(m.Content, "!present") {
		targetText, err := resolveTargetText(strings.TrimPrefix(m.Content, "!present"))
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "⚠️ "+err.Error())
			return
		}

		// Send a loading state (Gemini takes 2-3 seconds)
		loadingMsg, _ := s.ChannelMessageSend(m.ChannelID, "⏳ *The Oracle is peering into the syntax...*")

		ctx := context.Background()
		res, err := oracle.Present(ctx, targetText)

		// Delete loading message
		s.ChannelMessageDelete(m.ChannelID, loadingMsg.ID)

		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "⚠️ Oracle failure: "+err.Error())
			return
		}

		embed := &discordgo.MessageEmbed{
			Color:       0xC6D8F0, // Powder Blue
			Description: fmt.Sprintf("**『 %s 』**\n\n**Kana:** %s\n**Meaning:** %s", targetText, res.Reading, res.Translation),
		}
		s.ChannelMessageSendEmbed(m.ChannelID, embed)
		return
	}

	// !dissect Command
	if strings.HasPrefix(m.Content, "!dissect") {
		targetText, err := resolveTargetText(strings.TrimPrefix(m.Content, "!dissect"))
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "⚠️ "+err.Error())
			return
		}

		loadingMsg, _ := s.ChannelMessageSend(m.ChannelID, "👁️ *The Oracle is dissecting the linguistics...*")

		ctx := context.Background()
		res, err := oracle.Dissect(ctx, targetText)

		s.ChannelMessageDelete(m.ChannelID, loadingMsg.ID)

		if err != nil {
			// Catch rate limits gracefully
			if strings.Contains(err.Error(), "429") {
				s.ChannelMessageSend(m.ChannelID, "⚠️ The Oracle is overwhelmed by requests. Please wait a minute and try again.")
				return
			}
			s.ChannelMessageSend(m.ChannelID, "⚠️ Oracle failure: "+err.Error())
			return
		}

		embed := &discordgo.MessageEmbed{
			Title:       "👁️ Alchemical Dissection",
			Color:       0xC6D8F0, // Powder Blue
			Description: fmt.Sprintf("**Target:** %s\n**Kana:** %s\n**Translation:** %s", targetText, res.Reading, res.Translation),
			Fields: []*discordgo.MessageEmbedField{
				{Name: "JLPT Level", Value: res.JLPT, Inline: true},
				{Name: "Nuance & Context", Value: res.Nuance, Inline: false},
			},
		}

		// Format Grammar
		if len(res.Grammar) > 0 {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   "Grammar Architecture",
				Value:  "- " + strings.Join(res.Grammar, "\n- "),
				Inline: false,
			})
		}

		// Format Vocab
		if len(res.Vocab) > 0 {
			var vocabLines []string
			for _, v := range res.Vocab {
				vocabLines = append(vocabLines, fmt.Sprintf("• **%s** (%s): %s", v.Word, v.Reading, v.Meaning))
			}
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   "Morphological Breakdown",
				Value:  strings.Join(vocabLines, "\n"),
				Inline: false,
			})
		}

		s.ChannelMessageSendEmbed(m.ChannelID, embed)
		return
	}

	// --- SENTENCE CRUD: !delete ---
	if strings.HasPrefix(m.Content, "!delete ") {
		args := strings.TrimSpace(strings.TrimPrefix(m.Content, "!delete "))
		parts := strings.Fields(args)

		var targetVault string
		var targetIndex int
		var err error

		// Scenario A: Contextual delete while viewing an open vault (e.g., !delete 2)
		if len(parts) == 1 {
			targetIndex, err = strconv.Atoi(parts[0])
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ Provide a valid index number (e.g., `!delete 2`) or vault and index (`!delete Slang 2`).")
				return
			}

			cached, ok := vaultCache.Load(m.Author.ID)
			if !ok {
				s.ChannelMessageSend(m.ChannelID, "⚠️ No open grimoire context found. Use `!delete <grimoire> <index>` or run `!vault open <grimoire>` first.")
				return
			}

			session := cached.(VaultSession)
			targetVault = session.VaultName

			if targetIndex < 1 || targetIndex > len(session.Sentences) {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("⚠️ Index %d out of bounds for grimoire 『 %s 』.", targetIndex, targetVault))
				return
			}

			rawText := session.Sentences[targetIndex-1]

			if err := db.DeleteSentenceFromVault(m.Author.ID, targetVault, rawText); err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ "+err.Error())
				return
			}

			// Update the active cache by removing the deleted element
			updatedSentences := append(session.Sentences[:targetIndex-1], session.Sentences[targetIndex:]...)
			vaultCache.Store(m.Author.ID, VaultSession{
				VaultName: targetVault,
				Sentences: updatedSentences,
			})

			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🗡️ Purged fragment #%d from 『 %s 』.", targetIndex, targetVault))
			return
		}

		// Scenario B: Explicit vault and index specified (e.g., !delete Slang 2)
		if len(parts) >= 2 {
			targetVaultInput := parts[0]
			targetIndex, err = strconv.Atoi(parts[1])
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ Invalid index number specified.")
				return
			}

			targetVault, err = db.ResolveVaultInput(m.Author.ID, targetVaultInput)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ "+err.Error())
				return
			}

			// Re-fetch sentence list to map the requested index accurately
			sentences, err := db.OpenVault(m.Author.ID, targetVault)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ "+err.Error())
				return
			}

			if targetIndex < 1 || targetIndex > len(sentences) {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("⚠️ Index %d out of bounds for grimoire 『 %s 』.", targetIndex, targetVault))
				return
			}

			rawText := sentences[targetIndex-1]

			if err := db.DeleteSentenceFromVault(m.Author.ID, targetVault, rawText); err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ "+err.Error())
				return
			}

			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🗡️ Purged fragment #%d from 『 %s 』.", targetIndex, targetVault))
			return
		}
	}

	// --- SENTENCE CRUD: !clear ---
	if strings.HasPrefix(m.Content, "!clear ") {
		input := strings.TrimSpace(strings.TrimPrefix(m.Content, "!clear "))
		if input == "" {
			s.ChannelMessageSend(m.ChannelID, "⚠️ Usage: `!clear <grimoire_name_or_index>`")
			return
		}

		vaultName, err := db.ResolveVaultInput(m.Author.ID, input)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "⚠️ "+err.Error())
			return
		}

		count, err := db.ClearVaultSentences(m.Author.ID, vaultName)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "⚠️ Failed to clear grimoire: "+err.Error())
			return
		}

		// Evict cache if this vault is currently cached open
		if cached, ok := vaultCache.Load(m.Author.ID); ok {
			if cached.(VaultSession).VaultName == vaultName {
				vaultCache.Delete(m.Author.ID)
			}
		}

		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("火 Cleared **%d** fragment(s) from 『 %s 』.", count, vaultName))
		return
	}

	// --- GLOBAL SEARCH: !search ---
	if strings.HasPrefix(m.Content, "!search ") {
		args := strings.TrimSpace(strings.TrimPrefix(m.Content, "!search "))
		if args == "" {
			s.ChannelMessageSend(m.ChannelID, "⚠️ Usage: `!search <keyword> [page]`")
			return
		}

		parts := strings.Fields(args)
		page := 1
		keyword := args

		// Smart Parser: Check if the last word is a page number
		if len(parts) > 1 {
			lastPart := parts[len(parts)-1]
			if p, errConv := strconv.Atoi(lastPart); errConv == nil {
				page = p
				keyword = strings.TrimSpace(strings.TrimSuffix(args, lastPart))
			}
		}

		results, err := db.SearchSentences(m.Author.ID, keyword)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "⚠️ Search failed: "+err.Error())
			return
		}

		if len(results) == 0 {
			embed := &discordgo.MessageEmbed{
				Title:       fmt.Sprintf("🔍 Search Results: 『 %s 』", keyword),
				Description: "*No fragments found containing this keyword in any grimoire.*",
				Color:       0x4682B4, // Steel Blue
			}
			s.ChannelMessageSendEmbed(m.ChannelID, embed)
			return
		}

		// Extract just the raw text to feed into the Oracle cache
		var pureSentences []string
		for _, r := range results {
			pureSentences = append(pureSentences, r.RawText)
		}

		// SEED THE CACHE AS A PSEUDO-VAULT
		// This makes !present 1 or !dissect 2 work instantly on search results
		vaultCache.Store(m.Author.ID, VaultSession{
			VaultName: "Search: " + keyword,
			Sentences: pureSentences,
		})

		// --- Pagination Math ---
		perPage := 10
		maxPage := int(math.Ceil(float64(len(results)) / float64(perPage)))

		if page < 1 {
			page = 1
		}
		if page > maxPage {
			page = maxPage
		}

		start := (page - 1) * perPage
		end := start + perPage
		if end > len(results) {
			end = len(results)
		}

		displaySlice := results[start:end]

		// --- Build the Embed ---
		var descBuilder strings.Builder
		for i, res := range displaySlice {
			actualIndex := start + i + 1
			// Format: 1. [VaultName] Sentence
			descBuilder.WriteString(fmt.Sprintf("**%d.** [%s] %s\n\n", actualIndex, res.VaultName, res.RawText))
		}

		embed := &discordgo.MessageEmbed{
			Title:       fmt.Sprintf("🔍 Search Results: 『 %s 』", keyword),
			Description: descBuilder.String(),
			Color:       0x4682B4, // Steel Blue
			Footer: &discordgo.MessageEmbedFooter{
				Text: fmt.Sprintf("Page %d of %d • Total Matches: %d", page, maxPage, len(results)),
			},
		}

		s.ChannelMessageSendEmbed(m.ChannelID, embed)
		return
	}
}
