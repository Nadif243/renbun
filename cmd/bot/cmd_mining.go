package main

import (
	"bufio"
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"time"

	"rennbunengine/internal/db"
	"rennbunengine/internal/worker"

	"github.com/bwmarrin/discordgo"
)

func handleMine(s *discordgo.Session, m *discordgo.MessageCreate) {
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

func handleSave(s *discordgo.Session, m *discordgo.MessageCreate) {
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
			Color:       0x572E5B, // Deep Violet
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

	// Check if the scanner crashed during the loop
	if err := scanner.Err(); err != nil {
		fmt.Println("Warning: Error reading target.txt:", err)
	}

	return urls
}
