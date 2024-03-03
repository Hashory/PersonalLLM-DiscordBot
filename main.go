package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

func main() {
	// Setup logging.
	logFile, err := logSetup()
	if err != nil {
		log.Fatalf("Failed to setup logging: %v", err)
	}
	defer logFile.Close()

	// Load the bot configuration.
	config, err := LoadConfig("config.yml")
	if err != nil {
		fmt.Println("Failed to read `config.yml`: ", err)
		return
	}

	dg, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		fmt.Println("Failed to create the bot: ", err)
		return
	}

	// Create a new instance of BotService.
	bs := NewBotService(dg, config)

	// Register event handlers.
	bs.RegisterEventHandlers()

	if err = bs.Open(); err != nil {
		fmt.Println("Error opening connection: ", err)
		return
	}
	defer bs.Close()

	log.Println("Bot is starting..")
	fmt.Println("Bot is now running. Press CTRL+C to exit.")
	waitForExit()
	log.Println("Bot is stopping..")
}

func logSetup() (*os.File, error) {
	f, err := os.OpenFile("log.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	return f, nil
}

func waitForExit() {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)
	<-sc
}
