package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

var format = flag.String("format", "json", "output format: json or text")
var userAgent = flag.String("user-agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36", "override the User-Agent sent to the Discord REST API")

func main() {
	flag.Parse()

	switch *format {
	case "json", "text":
	default:
		log.Fatalf("invalid format %q: must be json or text", *format)
	}

	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		log.Fatal("DISCORD_TOKEN environment variable is required")
	}

	dg, err := discordgo.New(token)
	if err != nil {
		log.Fatalf("error creating discord session: %v", err)
	}

	if *userAgent != "" {
		dg.UserAgent = *userAgent
	}

	dg.AddHandler(messageCreate)

	dg.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentMessageContent

	err = dg.Open()
	if err != nil {
		log.Fatalf("error opening connection: %v", err)
	}
	defer dg.Close()

	log.Println("logging discord messages")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	channel := m.ChannelID
	if ch, err := s.State.Channel(m.ChannelID); err == nil {
		channel = ch.Name
	}

	guild := m.GuildID
	if guild == "" {
		guild = "DM"
	} else if g, err := s.State.Guild(m.GuildID); err == nil {
		guild = g.Name
	}

	if *format == "json" {
		msg := logMessage{
			Message:     m.Message,
			GuildName:   guild,
			ChannelName: channel,
		}
		b, err := json.Marshal(msg)
		if err != nil {
			log.Printf("error marshaling message: %v", err)
			return
		}
		fmt.Fprintln(os.Stdout, string(b))
		return
	}

	fmt.Fprintf(os.Stdout, "[%s #%s] %s: %s\n", guild, channel, m.Author.Username, m.Content)
}

type logMessage struct {
	*discordgo.Message
	GuildName   string `json:"guild_name"`
	ChannelName string `json:"channel_name"`
}
