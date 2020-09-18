package main

import (
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/worr/secstring"
	"os"
	"strconv"
)

func sendMessage(textmsg string) int {
	id, err := strconv.Atoi(os.Getenv("TELEGRAMM-CHANNEL"))
	if err != nil {
		print("Can't use TELEGRAMM-CHANNEL. Should be int but it's " + os.Getenv("TELEGRAMM-CHANNEL"))
		os.Exit(1)
	}
	token := os.Getenv("TELEGRAMM-TOKEN")
	ss, _ := secstring.FromString(&token)
	defer ss.Destroy()
	bot, err := tgbotapi.NewBotAPI(string(ss.String))
	if err != nil {
		print(err.Error())
		return 0
	}
	bot.Debug = false
	msg := tgbotapi.NewMessage(int64(id), textmsg)
	msg.DisableWebPagePreview = true
	msg.ParseMode = "HTML"
	Msgid, err := bot.Send(msg)
	if err != nil {
		emsg := tgbotapi.NewMessage(int64(id), err.Error())
		_, eerr := bot.Send(emsg)
		if eerr != nil {
			print(eerr.Error())
		}
		return 0
	}
	return Msgid.MessageID
}

func editMessage(mid int, textmsg string) {
	id, err := strconv.Atoi(os.Getenv("TELEGRAMM-CHANNEL"))
	if err != nil {
		print("Can't use TELEGRAMM-CHANNEL. Should be int but it's " + os.Getenv("TELEGRAMM-CHANNEL"))
		os.Exit(1)
	}
	token := os.Getenv("TELEGRAMM-TOKEN")
	ss, _ := secstring.FromString(&token)
	defer ss.Destroy()
	bot, err := tgbotapi.NewBotAPI(string(ss.String))
	if err != nil {
		print(err.Error())
	}
	bot.Debug = false
	msg := tgbotapi.NewEditMessageText(int64(id), mid, textmsg)
	msg.DisableWebPagePreview = true
	msg.ParseMode = "HTML"
	bot.Send(msg)
}
