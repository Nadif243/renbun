package main

import (
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

	// 2. Route Standard Commands
	content := m.Content
	switch {
	case content == "!ping":
		s.ChannelMessageSend(m.ChannelID, "Pong! 錬文 engine is listening.")
	case content == "!help":
		handleHelp(s, m)
	case content == "!bind":
		handleBind(s, m)
	case strings.HasPrefix(content, "!mine "):
		handleMine(s, m)
	case strings.HasPrefix(content, "!save"):
		handleSave(s, m)
	case strings.HasPrefix(content, "!vault"):
		handleVault(s, m)
	case strings.HasPrefix(content, "!present"):
		handlePresent(s, m)
	case strings.HasPrefix(content, "!dissect"):
		handleDissect(s, m)
	case strings.HasPrefix(content, "!delete "):
		handleDelete(s, m)
	case strings.HasPrefix(content, "!clear "):
		handleClear(s, m)
	case strings.HasPrefix(content, "!search "):
		handleSearch(s, m)
	case strings.HasPrefix(content, "!add "):
		handleAdd(s, m)
	}
}
