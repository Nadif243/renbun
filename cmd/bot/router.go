package main

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"rennbunengine/internal/db"

	"github.com/bwmarrin/discordgo"
)

// --- GLOBAL STATE & CACHE ---
type CacheEntry struct {
	Payload   []db.ExtractedSentence
	ExpiresAt time.Time
}
type VaultSession struct {
	VaultName string
	Sentences []string
}

var userState sync.Map
var mineCache sync.Map
var vaultCache sync.Map

// activeMenus stores the Message ID of active transient menus to supersede them
var activeMenus sync.Map

// --- THE SWITCHBOARD ---
// This replaces your old messageCreate. It acts as a traffic director.
func messageRouter(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	// 1. Check the State Interceptor first (If they are in the middle of a command)
	if intercepted := handleStateInterceptor(s, m); intercepted {
		return // Stop processing if the interceptor handled it
	}

	// 2. Fast exit if the message doesn't start with the prefix
	if !strings.HasPrefix(m.Content, "!") {
		return
	}

	// 3. Extract the exact command trigger (the first word)
	args := strings.Fields(m.Content)
	if len(args) == 0 {
		return
	}

	// Convert to lowercase so "!Vault" and "!vault" both work
	command := strings.ToLower(args[0])

	// 4. Route Standard Commands
	switch command {
	case "!ping":
		s.ChannelMessageSend(m.ChannelID, "Pong! 錬文 engine is listening.")

	case "!help", "!h":
		handleHelp(s, m)
	case "!bind":
		handleBind(s, m)
	case "!mine", "!m":
		handleMine(s, m)
	case "!vault", "!v":
		handleVault(s, m)
	case "!present", "!p":
		handlePresent(s, m)
	case "!dissect", "!d":
		handleDissect(s, m)
	case "!source", "!s":
		handleSource(s, m)

	case "!save":
		handleSave(s, m)
	case "!add":
		handleAdd(s, m)
	case "!search":
		handleSearch(s, m)
	case "!delete":
		handleDelete(s, m)
	case "!clear":
		handleClear(s, m)
	}
}

func handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// 1. Only process button clicks
	if i.Type != discordgo.InteractionMessageComponent {
		return
	}

	// 2. Get the User ID safely (works in servers and DMs)
	userID := ""
	if i.Member != nil {
		userID = i.Member.User.ID
	} else if i.User != nil {
		userID = i.User.ID
	}

	// 3. Load the vault state for this specific message
	val, ok := paginationCache.Load(i.Message.ID)
	if !ok {
		return // Ignore clicks on old/expired messages
	}
	state := val.(*PaginationState)

	// 4. Ensure only the vault owner can click the buttons
	if userID != state.AuthorID {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "⚠️ This is not your grimoire.",
				Flags:   discordgo.MessageFlagsEphemeral, // Only they see this warning
			},
		})
		return
	}

	customID := i.MessageComponentData().CustomID

	// 5. Handle the Close Button
	if customID == "page_close" {
		s.ChannelMessageDelete(i.ChannelID, i.Message.ID)
		paginationCache.Delete(i.Message.ID)
		return
	}

	// 6. Handle Pagination Math
	if customID == "page_next" && state.Page < state.MaxPage {
		state.Page++
	} else if customID == "page_prev" && state.Page > 1 {
		state.Page--
	}

	// 7. Rebuild the Slice
	start := (state.Page - 1) * 10 // Assuming 10 items per page
	end := start + 10
	if end > len(state.Sentences) {
		end = len(state.Sentences)
	}
	pageItems := state.Sentences[start:end]

	// 8. Rebuild the Embed Description
	var desc strings.Builder
	for idx, text := range pageItems {
		desc.WriteString(fmt.Sprintf("**%d.** %s\n\n", start+idx+1, text))
	}

	newEmbed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("📖 Vault Contents: 『 %s 』", state.VaultName),
		Description: desc.String(),
		Color:       0x572E5B,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Page %d of %d • Total Fragments: %d", state.Page, state.MaxPage, len(state.Sentences)),
		},
	}

	// 9. Rebuild the Buttons (to update their disabled state)
	newComponents := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "⬅️",
					Style:    discordgo.SecondaryButton,
					CustomID: "page_prev",
					Disabled: state.Page <= 1,
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
					Disabled: state.Page >= state.MaxPage,
				},
			},
		},
	}

	// 10. Instantly edit the message
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{newEmbed},
			Components: newComponents,
		},
	})
}

// SendTransientMenu sends an embed that supersedes older versions and self-destructs after 90s
func SendTransientMenu(s *discordgo.Session, channelID string, authorID string, menuKey string, embed *discordgo.MessageEmbed) {
	cacheKey := authorID + "_" + menuKey

	// 1. Supersede: If this user already has this menu open, delete the old one
	if oldMsgID, ok := activeMenus.Load(cacheKey); ok {
		err := s.ChannelMessageDelete(channelID, oldMsgID.(string))
		if err != nil {
			log.Printf("[Cleanup Error] Failed to delete old menu: %v\n", err)
		}
	}

	// 2. Send the new menu
	msg, err := s.ChannelMessageSendEmbed(channelID, embed)
	if err != nil {
		log.Printf("[Send Error] Failed to send new menu: %v\n", err)
		return
	}

	// 3. Cache the new message ID
	activeMenus.Store(cacheKey, msg.ID)

	// 4. The 90-Second Fade: Launch a background timer to delete it
	go func(mID string) {
		time.Sleep(90 * time.Second)
		// Only attempt API deletion if this is STILL the active menu in the cache
		if currentID, ok := activeMenus.Load(cacheKey); ok && currentID.(string) == mID {
			err := s.ChannelMessageDelete(channelID, mID)

			// Ignore 10008 (Unknown Message) errors in case the user manually deleted it
			if err != nil && !strings.Contains(err.Error(), "10008") {
				log.Printf("[Fade Error] Failed to delete expired menu: %v\n", err)
			}

			activeMenus.Delete(cacheKey)
		}
	}(msg.ID)
}
