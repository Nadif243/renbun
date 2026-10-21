package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"rennbunengine/internal/db"
	"rennbunengine/internal/oracle"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

func main() {
	// 1. Load Environment
	if err := godotenv.Load(".env"); err != nil {
		log.Println("Warning: No .env file found.")
	}

	// 2. Ignite the Crucible (DB)
	db.InitDB("./renbun.db")
	defer db.DB.Close()

	// Inject default target URLs if the sources table is empty
	if err := db.SeedDefaultSources(); err != nil {
		log.Fatal("Critical: Failed to seed default sources: ", err)
	}

	// 3. Ignite the Oracle
	geminiKey := os.Getenv("GEMINI_API_KEY")
	if geminiKey == "" {
		log.Fatal("Critical: GEMINI_API_KEY is missing from the void (.env)")
	}
	if err := oracle.InitOracle(geminiKey); err != nil {
		log.Fatal(err)
	}

	// 4. Initialize Discord Session
	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		log.Fatal("CRITICAL: DISCORD_TOKEN is missing!")
	}
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatal("Error creating Discord session: ", err)
	}

	// 5. Register the Router
	dg.AddHandler(messageRouter)
	dg.AddHandler(handleInteraction)
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentMessageContent

	// 6. Open Connection & Set Status
	if err = dg.Open(); err != nil {
		log.Fatal("Error opening connection: ", err)
	}
	dg.UpdateStatusComplex(discordgo.UpdateStatusData{
		Activities: []*discordgo.Activity{{
			Type:  discordgo.ActivityTypeCustom,
			Name:  "Custom Status",
			State: "錬金術駆動の没入型エンジンで言葉を錬成中",
		}},
	})

	// 7. Keep Alive & Graceful Shutdown
	fmt.Println("Renbun Engine (錬文) v1.2.0 is online. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	fmt.Println("\nShutting down Renbun Engine safely...")
	dg.Close()
}
