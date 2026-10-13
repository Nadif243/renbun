package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"rennbunengine/internal/db"

	"github.com/bwmarrin/discordgo"
)

func handleHelp(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Open the local image file
	file, err := os.Open("assets/logo.png")
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "⚠️ System Error: Missing grimoire seal (logo.png).")
		return
	}
	defer file.Close()

	embed := &discordgo.MessageEmbed{
		Title:       "📜 Renbun (錬文) - The Alchemical Grimoire",
		Description: "A Japanese language study engine for sentence extraction, morphological dissection, and vault management.\n\n",
		Color:       0x8B9EB6, // Heather Blue
		Author: &discordgo.MessageEmbedAuthor{
			Name: "錬文",
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: "attachment://logo.png",
		},

		Fields: []*discordgo.MessageEmbedField{
			// ROW 1
			{
				Name: "🔍 Scrying & Acquisition",
				Value: "`!mine <keyword> [in <category>]`\nScour the web (or specific domains) for high-quality sentences.\n\n" +
					"`!save <1 2 3> to <vault>`\nBind mined fragments into a grimoire.\n\n" +
					"`!add <vault> <sentence>`\nManually inject a raw sentence into a grimoire.",
				Inline: true,
			},
			{
				Name: "🌐 Target Domains (Sources)",
				Value: "`!source list`\nView your active domain categories.\n\n" +
					"`!source add <category> <url>`\nInject a new scraping target.\n\n" +
					"`!source remove <category> <index>`\nRemove a target from a category.",
				Inline: true,
			},
			// THE INVISIBLE LINE BREAK
			{
				Name:   "\u200b",
				Value:  "\u200b",
				Inline: true,
			},
			// ROW 2
			{
				Name: "📖 Grimoire Management",
				Value: "`!vault list` | `!vault create <name>` | `!vault delete <name>`\nManage your core vaults.\n\n" +
					"`!vault open <name> [page]`\nOpen a grimoire to view its contents.\n\n" +
					"`!search <keyword> [page]`\nHunt for a specific word across all your grimoires.\n\n" +
					"`!delete <index>` OR `!delete <vault> <index>`\nPurge a specific fragment.\n\n" +
					"`!clear <vault>`\nErase all fragments within a grimoire.",
				Inline: true, // Switched back to true
			},
			{
				Name: "👁️ The Oracle (Inspection)",
				Value: "*Note: You must have a grimoire opened (via `!vault open` or `!search`) to use the Oracle.*\n\n" +
					"`!present <index/text>`\nReveal the kana reading and English translation.\n\n" +
					"`!dissect <index/text>`\nPerform a deep morphological breakdown.",
				Inline: true, // Switched back to true
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Renbun Engine • Built for natural immersion study.",
		},
	}

	// Send as a Complex Message with the file payload
	s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{embed},
		Files: []*discordgo.File{
			{
				Name:   "logo.png",
				Reader: file,
			},
		},
	})
}

func handleBind(s *discordgo.Session, m *discordgo.MessageCreate) {
	response, err := db.BindUser(m.Author.ID)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "⚠️ Alchemical failure during binding: "+err.Error())
		return
	}
	s.ChannelMessageSend(m.ChannelID, "🩸 "+response)
}

func handleSearch(s *discordgo.Session, m *discordgo.MessageCreate) {
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
			Color:       0x8B9EB6,
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
		Color:       0x8B9EB6,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Page %d of %d • Total Matches: %d", page, maxPage, len(results)),
		},
	}

	s.ChannelMessageSendEmbed(m.ChannelID, embed)
}

func handleStateInterceptor(s *discordgo.Session, m *discordgo.MessageCreate) bool {
	if state, ok := userState.Load(m.Author.ID); ok {
		input := strings.TrimSpace(m.Content)

		// Global cancel catch
		if input == "cancel" {
			userState.Delete(m.Author.ID)
			s.ChannelMessageSend(m.ChannelID, "Action aborted.")
			return true
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
			return true
		}

		// State: Opening a Vault
		if stateStr == "awaiting_vault_open_name" {
			vaultName, err := db.ResolveVaultInput(m.Author.ID, input)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ "+err.Error())
				userState.Delete(m.Author.ID)
				return true
			}

			sentences, err := db.OpenVault(m.Author.ID, vaultName)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ Failed to open vault: "+err.Error())
				userState.Delete(m.Author.ID)
				return true
			}

			embed := &discordgo.MessageEmbed{
				Title: "📖 Vault Contents: 『 " + vaultName + " 』",
				Color: 0x572E5B, // Deep Violet
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
			return true
		}

		// State: Deleting a Vault
		if stateStr == "awaiting_vault_delete_name" {
			vaultName, err := db.ResolveVaultInput(m.Author.ID, input)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ "+err.Error())
				userState.Delete(m.Author.ID)
				return true
			}

			if vaultName == "Primary Grimoire" {
				s.ChannelMessageSend(m.ChannelID, "⚠️ The Primary Grimoire cannot be destroyed.")
				userState.Delete(m.Author.ID)
				return true
			}

			response, err := db.DeleteVault(m.Author.ID, vaultName)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "⚠️ Failed to incinerate vault: "+err.Error())
			} else {
				s.ChannelMessageSend(m.ChannelID, response)
			}

			userState.Delete(m.Author.ID)
			return true
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
				return true
			}

			// Pull from cache
			cachedData, ok := mineCache.Load(m.Author.ID)
			if !ok {
				s.ChannelMessageSend(m.ChannelID, "⚠️ Cache expired. Mine again.")
				userState.Delete(m.Author.ID)
				return true
			}

			entry := cachedData.(CacheEntry)
			if time.Now().After(entry.ExpiresAt) {
				mineCache.Delete(m.Author.ID)
				s.ChannelMessageSend(m.ChannelID, "⚠️ Your mined fragments have decayed. Please run `!mine` again.")
				userState.Delete(m.Author.ID)
				return true
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
			return true
		}
	}
	return false
}
