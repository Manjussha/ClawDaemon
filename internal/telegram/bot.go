// Package telegram provides the Telegram bot for admin notifications and commands.
package telegram

import (
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot wraps the Telegram bot API.
type Bot struct {
	api        *tgbotapi.BotAPI
	adminChatID int64
	handler    *CommandHandler
}

// New creates a Bot. Returns nil if token is empty (Telegram disabled).
func New(token string, adminChatID int64, handler *CommandHandler) (*Bot, error) {
	if token == "" {
		return nil, nil
	}
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("telegram.New: %w", err)
	}
	b := &Bot{api: api, adminChatID: adminChatID, handler: handler}
	if handler != nil {
		handler.bot = b
	}
	return b, nil
}

// Send sends a plain text message to the admin chat.
func (b *Bot) Send(msg string) error {
	if b == nil {
		return nil
	}
	m := tgbotapi.NewMessage(b.adminChatID, msg)
	m.ParseMode = "Markdown"
	_, err := b.api.Send(m)
	if err != nil {
		return fmt.Errorf("telegram.Send: %w", err)
	}
	return nil
}

// SendLimitAlert sends a rate-limit alert with inline action buttons.
func (b *Bot) SendLimitAlert(workerName, taskTitle string, workerID, taskID int) error {
	if b == nil {
		return nil
	}
	text := fmt.Sprintf("⚠️ *Rate limit detected!*\n\nWorker: %s\nTask: %s\n\nChoose an action:",
		workerName, taskTitle)
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Switch CLI", fmt.Sprintf("switch_%d", workerID)),
			tgbotapi.NewInlineKeyboardButtonData("⏳ Wait 1h", fmt.Sprintf("wait_%d", taskID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⏭ Skip Task", fmt.Sprintf("skip_%d", taskID)),
			tgbotapi.NewInlineKeyboardButtonData("⏹ Pause All", "pause_all"),
		),
	)
	msg := tgbotapi.NewMessage(b.adminChatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	_, err := b.api.Send(msg)
	if err != nil {
		return fmt.Errorf("telegram.SendLimitAlert: %w", err)
	}
	return nil
}

// Start begins polling for updates. Must be called in a goroutine.
// Only processes messages from adminChatID.
func (b *Bot) Start(ctx interface{ Done() <-chan struct{} }) {
	if b == nil {
		return
	}
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			b.api.StopReceivingUpdates()
			return
		case update, ok := <-updates:
			if !ok {
				return
			}
			// Only handle admin chat.
			if update.Message != nil && update.Message.Chat.ID != b.adminChatID {
				continue
			}
			if update.CallbackQuery != nil && update.CallbackQuery.From != nil {
				// Can't restrict callback by chat, but only admin would have these buttons.
				b.handleCallback(update.CallbackQuery)
				continue
			}
			if update.Message != nil && b.handler != nil {
				b.handler.Handle(update.Message)
			}
		}
	}
}

func (b *Bot) handleCallback(query *tgbotapi.CallbackQuery) {
	data := query.Data
	if b.handler != nil {
		b.handler.HandleCallback(data, query.ID)
	}
	// Acknowledge the callback.
	ack := tgbotapi.NewCallback(query.ID, "")
	if _, err := b.api.Request(ack); err != nil {
		log.Printf("telegram: ack callback: %v", err)
	}
}

// reply sends a text reply to a message.
func (b *Bot) reply(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("telegram.reply: %v", err)
	}
}
