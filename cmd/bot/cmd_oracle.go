package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"rennbunengine/internal/oracle"

	"github.com/bwmarrin/discordgo"
)

func handlePresent(s *discordgo.Session, m *discordgo.MessageCreate) {
	parts := strings.SplitN(m.Content, " ", 2)
	input := ""
	if len(parts) == 2 {
		input = parts[1]
	}

	targetText, err := resolveTargetText(m.Author.ID, input)
	if err != nil {
		s.MessageReactionAdd(m.ChannelID, m.ID, "⚠️")
		s.ChannelMessageSend(m.ChannelID, "⚠️ "+err.Error())
		return
	}

	// 1. Instantly react with an hourglass
	s.MessageReactionAdd(m.ChannelID, m.ID, "⏳")

	ctx := context.Background()
	res, err := oracle.Present(ctx, targetText)

	// 2. Remove the hourglass when the Oracle returns
	s.MessageReactionRemove(m.ChannelID, m.ID, "⏳", s.State.User.ID)

	if err != nil {
		// 3a. React with a warning on failure
		s.MessageReactionAdd(m.ChannelID, m.ID, "⚠️")
		s.ChannelMessageSend(m.ChannelID, "⚠️ Oracle failure: "+err.Error())
		return
	}

	// 3b. React with a checkmark on success
	s.MessageReactionAdd(m.ChannelID, m.ID, "✅")

	embed := &discordgo.MessageEmbed{
		Color:       0x950B10, // Dark Crimson
		Description: fmt.Sprintf("**『 %s 』**\n\n**Kana:** %s\n**Meaning:** %s", targetText, res.Reading, res.Translation),
	}
	s.ChannelMessageSendEmbed(m.ChannelID, embed)
}

func handleDissect(s *discordgo.Session, m *discordgo.MessageCreate) {
	parts := strings.SplitN(m.Content, " ", 2)
	input := ""
	if len(parts) == 2 {
		input = parts[1]
	}

	targetText, err := resolveTargetText(m.Author.ID, input)
	if err != nil {
		s.MessageReactionAdd(m.ChannelID, m.ID, "⚠️")
		s.ChannelMessageSend(m.ChannelID, "⚠️ "+err.Error())
		return
	}

	s.MessageReactionAdd(m.ChannelID, m.ID, "⏳")

	ctx := context.Background()
	res, err := oracle.Dissect(ctx, targetText)

	s.MessageReactionRemove(m.ChannelID, m.ID, "⏳", s.State.User.ID)

	if err != nil {
		// Catch rate limits gracefully
		if strings.Contains(err.Error(), "429") {
			s.MessageReactionAdd(m.ChannelID, m.ID, "⚠️")
			s.ChannelMessageSend(m.ChannelID, "⚠️ The Oracle is overwhelmed by requests. Please wait a minute and try again.")
			return
		}
		s.MessageReactionAdd(m.ChannelID, m.ID, "⚠️")
		s.ChannelMessageSend(m.ChannelID, "⚠️ Oracle failure: "+err.Error())
		return
	}

	s.MessageReactionAdd(m.ChannelID, m.ID, "✅")

	embed := &discordgo.MessageEmbed{
		Title:       "👁️ Alchemical Dissection",
		Color:       0x950B10,
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
}

// Helper function to resolve the target text (Index vs Raw Text)
func resolveTargetText(authorID string, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("provide a vault index (e.g., `1`) or raw Japanese text.")
	}

	// If they typed a number, pull from vaultCache
	if idx, err := strconv.Atoi(input); err == nil {
		// 1. Check the Mining Cache First (Highly transient)
		if cachedMine, ok := mineCache.Load(authorID); ok {
			entry := cachedMine.(CacheEntry)
			// Verify the mined fragments haven't decayed
			if !time.Now().After(entry.ExpiresAt) {
				extractedList := entry.Payload
				// Ensure the index exists in the mined list
				if idx >= 1 && idx <= len(extractedList) {
					return extractedList[idx-1].RawText, nil // Target the RawText field
				}
			}
		}

		// 2. Fallback to the Vault Cache
		if cachedVault, ok := vaultCache.Load(authorID); ok {
			session := cachedVault.(VaultSession)
			sentences := session.Sentences
			if idx >= 1 && idx <= len(sentences) {
				return sentences[idx-1], nil
			}
		}

		return "", fmt.Errorf("index %d does not exist in your active mine or open grimoire.", idx)
	}

	// If it's not a number, assume they typed raw Japanese text
	return input, nil
}
