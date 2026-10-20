package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"rennbunengine/internal/db"

	"github.com/bwmarrin/discordgo"
)

type PaginationState struct {
	AuthorID  string
	VaultName string
	Sentences []string
	Page      int
	MaxPage   int
}

// Stores active grimoire pages mapped by MessageID
var paginationCache sync.Map

func handleVault(s *discordgo.Session, m *discordgo.MessageCreate) {
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
			Color: 0x572E5B, // Deep Violet
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
			Color:       0x572E5B,
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
				Color:       0x572E5B,
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
				Color:       0x572E5B,
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
			Color:       0x572E5B,
			Footer: &discordgo.MessageEmbedFooter{
				Text: fmt.Sprintf("Page %d of %d • Total Fragments: %d", page, maxPage, len(sentences)),
			},
		}

		// 1. Build the buttons using maxPage
		components := []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "⬅️",
						Style:    discordgo.SecondaryButton,
						CustomID: "page_prev",
						Disabled: page <= 1,
					},
					discordgo.Button{
						Label:    "✖️",
						Style:    discordgo.DangerButton,
						CustomID: "page_close",
					},
					discordgo.Button{
						Label:    "➡️",
						Style:    discordgo.SecondaryButton,
						CustomID: "page_next",
						Disabled: page >= maxPage,
					},
				},
			},
		}

		// 2. Send the embed AND the buttons together
		msg, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
		})
		if err != nil {
			return
		}

		// 3. Save the state so the buttons know what to flip
		paginationCache.Store(msg.ID, &PaginationState{
			AuthorID:  m.Author.ID,
			VaultName: vaultName,
			Sentences: sentences,
			Page:      page,
			MaxPage:   maxPage,
		})
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
				Color:       0x572E5B,
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
}

func handleDelete(s *discordgo.Session, m *discordgo.MessageCreate) {
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

func handleClear(s *discordgo.Session, m *discordgo.MessageCreate) {
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
}

func handleAdd(s *discordgo.Session, m *discordgo.MessageCreate) {
	input := strings.TrimSpace(strings.TrimPrefix(m.Content, "!add "))

	// SplitN with '2' forces Go to only split at the FIRST space.
	// parts[0] becomes the vault. parts[1] becomes the entire rest of the message.
	parts := strings.SplitN(input, " ", 2)

	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		s.ChannelMessageSend(m.ChannelID, "⚠️ Usage: `!add <grimoire_name_or_index> <sentence>`")
		return
	}

	vaultTarget := parts[0]
	rawSentence := strings.TrimSpace(parts[1])

	vaultName, err := db.ResolveVaultInput(m.Author.ID, vaultTarget)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "⚠️ "+err.Error())
		return
	}

	// Wrap the single string into the ExtractedSentence slice format your DB expects
	sentenceData := []db.ExtractedSentence{
		{
			Keyword: "Manual Entry",
			RawText: rawSentence,
			Source:  "Manual", // Marks it so user know it didn't come from a website
		},
	}

	addedCount, err := db.SaveToVault(m.Author.ID, vaultName, sentenceData)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "⚠️ Failed to bind fragment: "+err.Error())
		return
	}

	if addedCount == 0 {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🛡️ 『 %s 』: This fragment already exists here.", vaultName))
		return
	}

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🩸 Successfully bound **1** manual fragment to 『 %s 』.", vaultName))
}
