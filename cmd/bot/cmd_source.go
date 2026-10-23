package main

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"

	"rennbunengine/internal/db"

	"github.com/bwmarrin/discordgo"
)

// prettyURL decodes percent-escapes (e.g. %E7%A7%91 -> 科) so Japanese links stay readable.
// Falls back to the raw string if it can't be decoded.
func prettyURL(raw string) string {
	if decoded, err := url.PathUnescape(raw); err == nil {
		return decoded
	}
	return raw
}

func handleSource(s *discordgo.Session, m *discordgo.MessageCreate) {
	args := strings.Fields(m.Content)
	if len(args) < 2 {
		s.ChannelMessageSend(m.ChannelID, "⚠️ Usage: `!source list`, `!source open <category> [page]`, `!source add <category> <url>`, or `!source remove <category> <index>`")
		return
	}

	subCommand := args[1]

	switch subCommand {
	case "list":
		cats, err := db.ListCategories()
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "⚠️ Failed to fetch sources: "+err.Error())
			return
		}

		if len(cats) == 0 {
			s.ChannelMessageSend(m.ChannelID, "No sources forged yet.")
			return
		}

		var description strings.Builder
		for i, c := range cats {
			description.WriteString(fmt.Sprintf("**%d.** %s `[%d targets]`\n\n", i+1, c.Name, c.Count))
		}

		embed := &discordgo.MessageEmbed{
			Title:       "🌐 Target Grimoires (Sources)",
			Description: description.String(),
			Color:       0x8B9EB6, // Heather Blue
			Footer: &discordgo.MessageEmbedFooter{
				Text: "Inspect one with !source open <name or index>",
			},
		}
		SendTransientMenu(s, m.ChannelID, m.Author.ID, "source_list", embed)

	case "open":
		if len(args) < 3 {
			s.ChannelMessageSend(m.ChannelID, "⚠️ Usage: `!source open <category or index> [page]`")
			return
		}

		// Smart parser (same idea as !vault open): if the whole input isn't a category,
		// treat the last word as a page number.
		page := 1
		category, err := db.ResolveCategoryInput(strings.Join(args[2:], " "))
		if err != nil && len(args) > 3 {
			if p, convErr := strconv.Atoi(args[len(args)-1]); convErr == nil {
				page = p
				category, err = db.ResolveCategoryInput(strings.Join(args[2:len(args)-1], " "))
			}
		}
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "⚠️ "+err.Error())
			return
		}

		urls, err := db.ListCategoryUrls(category)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "⚠️ Failed to read targets: "+err.Error())
			return
		}

		// --- Pagination Math ---
		perPage := 10
		maxPage := max(int(math.Ceil(float64(len(urls))/float64(perPage))), 1)
		page = min(max(page, 1), maxPage) // snap out-of-range pages

		start := (page - 1) * perPage
		end := min(start+perPage, len(urls))

		var desc strings.Builder
		for i, u := range urls[start:end] {
			// Index shown here is exactly what `!source remove <category> <index>` expects
			desc.WriteString(fmt.Sprintf("**%d.** `%s`\n", start+i+1, prettyURL(u)))
		}

		embed := &discordgo.MessageEmbed{
			Title:       fmt.Sprintf("🌐 Targets: 『 %s 』", category),
			Description: strings.TrimSpace(desc.String()),
			Color:       0x8B9EB6, // Heather Blue
			Footer: &discordgo.MessageEmbedFooter{
				Text: fmt.Sprintf("Page %d of %d • Total Targets: %d", page, maxPage, len(urls)),
			},
		}
		SendTransientMenu(s, m.ChannelID, m.Author.ID, "source_open", embed)

	case "add":
		if len(args) < 4 {
			s.ChannelMessageSend(m.ChannelID, "⚠️ Usage: `!source add <category> <url>`")
			return
		}
		// The URL is always the last argument; everything before it is the category
		target := args[len(args)-1]
		category := strings.Join(args[2:len(args)-1], " ")

		if err := db.AddSource(category, target); err != nil {
			s.ChannelMessageSend(m.ChannelID, "⚠️ Failed to add source: "+err.Error())
			return
		}
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("✅ Added target to **%s**.", category))

	case "remove":
		if len(args) < 4 {
			s.ChannelMessageSend(m.ChannelID, "⚠️ Usage: `!source remove <category or index> <target index>`")
			return
		}

		// The target index is always the last argument; everything before it is the category (name or number)
		idx, err := strconv.Atoi(args[len(args)-1])
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "⚠️ Index must be a number.")
			return
		}

		category, err := db.ResolveCategoryInput(strings.Join(args[2:len(args)-1], " "))
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "⚠️ "+err.Error())
			return
		}

		if err := db.RemoveSource(category, idx); err != nil {
			s.ChannelMessageSend(m.ChannelID, "⚠️ Failed to remove source: "+err.Error())
			return
		}
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("✅ Removed target `%d` from **%s**.", idx, category))

	default:
		s.ChannelMessageSend(m.ChannelID, "⚠️ Unknown command. Use list, open, add, or remove.")
	}
}
