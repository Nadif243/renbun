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
