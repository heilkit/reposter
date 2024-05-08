// flop-reposter simply redirects content from master channels to chats. All to all.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/heilkit/tg"
	"github.com/heilkit/tg/middleware"
	"github.com/heilkit/tg/scheduler"
	"log"
	"log/slog"
	"os"
	"strconv"
)

type Config struct {
	Token    string  `json:"token"`
	Admins   []int64 `json:"admin-list"`
	Channels []int64 `json:"channel-ids"`
	Chats    []int64 `json:"list-of-chats"`
	Filename string  `json:"-"`
}

func contains(list []int64, el int64) bool {
	for _, a := range list {
		if a == el {
			return true
		}
	}
	return false
}

func ConfigFromFile(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()
	var config Config
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}
	config.Filename = filename
	return &config, nil
}

func (cfg *Config) Save() error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("error saving config: %w", err)
	}
	if err := os.WriteFile(cfg.Filename, data, 0666); err != nil {
		return fmt.Errorf("error saving config: %w", err)
	}
	return nil
}

func (cfg *Config) AddChannel(channel int64) error {
	cfg.Channels = append(cfg.Channels, channel)
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("error adding channel: %w (%d)", err, channel)
	}
	return nil
}

func (cfg *Config) AddChat(chat int64) error {
	cfg.Chats = append(cfg.Chats, chat)
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("error adding chat: %w (%d)", err, chat)
	}
	return nil
}

func (cfg *Config) AddAdmin(adm int64) error {
	cfg.Admins = append(cfg.Admins, adm)
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("error adding admin: %w (%d)", err, adm)
	}
	return nil
}

func main() {
	cfgPath := flag.String("config", "config.json", "path to config file")
	flag.Parse()
	cfg, err := ConfigFromFile(*cfgPath)
	if err != nil {
		log.Fatal(err)
	}

	bot, err := tg.NewBot(tg.Settings{
		Token:       cfg.Token,
		Synchronous: true,
		OnError:     tg.OnErrorLog(),
		Scheduler:   scheduler.ExtraConservative(),
		Retries:     3,
	})
	if err != nil {
		log.Fatal(err)
	}

	admins := bot.Group(middleware.Personal(), func(handlerFunc tg.HandlerFunc) tg.HandlerFunc {
		return func(ctx tg.Context) error {
			if ctx == nil || ctx.Chat() == nil || len(cfg.Admins) == 0 || contains(cfg.Admins, ctx.Chat().ID) {
				return handlerFunc(ctx)
			}
			return nil
		}
	})

	admins.Handle("/help", func(ctx tg.Context) error {
		return ctx.Reply(fmt.Sprintf(
			"Admins: %d, Channels: %d, Chats: %d,\n"+
				"/admin [id]\n"+
				"/chan [id]\n"+
				"/chat [id]",
			len(cfg.Admins), len(cfg.Channels), len(cfg.Chats),
		))
	})

	admins.Handle("/admin", func(ctx tg.Context) error {
		if len(ctx.Args()) == 0 {
			return ctx.Reply("no id, provide it in the same message, /help?")
		}
		adm, err := strconv.ParseInt(ctx.Args()[0], 10, 64)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("error parsing id"))
		}
		if err := cfg.AddAdmin(adm); err != nil {
			return ctx.Reply("error addng to cfg")
		}
		return bot.React(ctx.Message(), tg.ReactionLike)
	})

	admins.Handle("/chan", func(ctx tg.Context) error {
		if len(ctx.Args()) == 0 {
			return ctx.Reply("no id, provide it in the same message, /help?")
		}
		adm, err := strconv.ParseInt(ctx.Args()[0], 10, 64)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("error parsing id"))
		}
		if err := cfg.AddChannel(adm); err != nil {
			return ctx.Reply("error addng to cfg")
		}
		return bot.React(ctx.Message(), tg.ReactionLike)
	})

	addChat := func(ctx tg.Context) error {
		if len(ctx.Args()) == 0 {
			return ctx.Reply("no id, provide it in the same message, /help?")
		}
		adm, err := strconv.ParseInt(ctx.Args()[0], 10, 64)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("error parsing id"))
		}
		if err := cfg.AddChat(adm); err != nil {
			return ctx.Reply("error addng to cfg")
		}
		return bot.React(ctx.Message(), tg.ReactionLike)
	}
	admins.Handle("/chat", addChat)
	admins.Handle("/add", addChat)

	admins.Handle("/shutdown", func(ctx tg.Context) error {
		if len(ctx.Args()) == 0 {
			return ctx.Reply("no id, send /shutdown with your id to shutdown, /help?")
		}
		bot.Stop()
		return nil
	})

	admins.Handle("/ls", func(ctx tg.Context) error {
		data, err := json.MarshalIndent(cfg.Chats, "", "  ")
		if err != nil {
			return bot.React(ctx.Message(), tg.ReactionDislike)
		}
		return ctx.Reply(string(data))
	})

	bot.Handle("/ok", func(ctx tg.Context) error {
		return ctx.Reply("OK")
	})

	forward := func(cs tg.Contexts) error {
		if !contains(cfg.Channels, cs.Chat().ID) {
			return nil
		}
		for _, chat := range cfg.Chats {
			if err := cs.ForwardTo(&tg.Chat{ID: chat}); err != nil {
				slog.Warn("err while forwarding", "err", err, "id", chat)
			}
		}
		return nil
	}

	bot.Group(middleware.Public()).HandleAlbum(forward, tg.OnChannelPost)
	bot.Group(middleware.Public()).Handle(tg.OnText, func(ctx tg.Context) error {
		return forward(tg.Contexts{ctx})
	})

	bot.Start()
}
