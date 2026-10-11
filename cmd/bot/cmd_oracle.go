package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"rennbunengine/internal/oracle"

	"github.com/bwmarrin/discordgo"
)

func handlePresent(s *discordgo.Session, m *discordgo.MessageCreate) {
	targetText, err := resolveTargetText(m.Author.ID, strings.TrimPrefix(m.Content, "!present"))
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
}

func handleDissect(s *discordgo.Session, m *discordgo.MessageCreate) {
	targetText, err := resolveTargetText(m.Author.ID, strings.TrimPrefix(m.Content, "!dissect"))
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
}

// Helper function to resolve the target text (Index vs Raw Text)
func resolveTargetText(authorID string, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("provide a vault index (e.g., `1`) or raw Japanese text.")
	}

	// If they typed a number, pull from vaultCache
	if idx, err := strconv.Atoi(input); err == nil {
		cachedData, ok := vaultCache.Load(authorID) // Note: using authorID here
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
