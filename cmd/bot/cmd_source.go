package main

import (
	"fmt"
	"strconv"
	"strings"

	"rennbunengine/internal/db"

	"github.com/bwmarrin/discordgo"
)

func handleSource(s *discordgo.Session, m *discordgo.MessageCreate) {
	args := strings.Fields(m.Content)
	if len(args) < 2 {
		s.ChannelMessageSend(m.ChannelID, "⚠️ Usage: `!source list`, `!source add <category> <url>`, or `!source remove <category> <index>`")
		return
	}

	subCommand := args[1]

	switch subCommand {
	case "list":
		sources, err := db.ListSources()
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "⚠️ Failed to fetch sources: "+err.Error())
			return
		}

		if len(sources) == 0 {
			s.ChannelMessageSend(m.ChannelID, "No sources forged yet.")
			return
		}

		var description strings.Builder
		idx := 1
		for cat, urls := range sources {
			description.WriteString(fmt.Sprintf("**%d.** %s `[%d targets]`\n\n", idx, cat, len(urls)))
			idx++
		}

		embed := &discordgo.MessageEmbed{
			Title:       "🌐 Target Grimoires (Sources)",
			Description: description.String(),
			Color:       0x8B9EB6, // Heather Blue
		}
		s.ChannelMessageSendEmbed(m.ChannelID, embed)

	case "add":
		if len(args) < 4 {
			s.ChannelMessageSend(m.ChannelID, "⚠️ Usage: `!source add <category> <url>`")
			return
		}
		// The URL is always the last argument; everything before it is the category
		url := args[len(args)-1]
		category := strings.Join(args[2:len(args)-1], " ")

		if err := db.AddSource(category, url); err != nil {
			s.ChannelMessageSend(m.ChannelID, "⚠️ Failed to add source: "+err.Error())
			return
		}
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("✅ Added target to **%s**.", category))

	case "remove":
		if len(args) < 4 {
			s.ChannelMessageSend(m.ChannelID, "⚠️ Usage: `!source remove <category> <index>`")
			return
		}
		// The Index is always the last argument; everything before it is the category
		indexStr := args[len(args)-1]
		category := strings.Join(args[2:len(args)-1], " ")

		idx, err := strconv.Atoi(indexStr)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "⚠️ Index must be a number.")
			return
		}

		if err := db.RemoveSource(category, idx); err != nil {
			s.ChannelMessageSend(m.ChannelID, "⚠️ Failed to remove source: "+err.Error())
			return
		}
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("✅ Removed target `%d` from **%s**.", idx, category))

	default:
		s.ChannelMessageSend(m.ChannelID, "⚠️ Unknown command. Use list, add, or remove.")
	}
}
