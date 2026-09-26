package telegram

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"spikeidx/internal/domain"
)

type Notifier struct {
	BotToken string
	ChatID   string
	client   *http.Client
}

func New(botToken, chatID string) *Notifier {
	return &Notifier{BotToken: botToken, ChatID: chatID, client: &http.Client{Timeout: 15 * time.Second}}
}

func (n *Notifier) IsEnabled() bool { return n.BotToken != "" && n.ChatID != "" }

func compact(v int64) string {
	switch {
	case v >= 1_000_000_000:
		return fmt.Sprintf("%.1fM", float64(v)/1_000_000_000)
	case v >= 1_000_000:
		g := float64(v) / 1_000_000
		if g == float64(int64(g)) {
			return fmt.Sprintf("%djt", int64(g))
		}
		return fmt.Sprintf("%.1fjt", g)
	case v >= 1_000:
		return fmt.Sprintf("%drb", v/1_000)
	default:
		return fmt.Sprintf("%d", v)
	}
}

func Digest(date string, signals []domain.Signal) string {
	if len(signals) == 0 {
		return fmt.Sprintf("SpikeIDX %s: No spike today.", date)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "🚨 Spike %s (%d):\n", date, len(signals))
	for _, s := range signals {
		fmt.Fprintf(&b, "%s %s (%.1fx avg20) %+.1f%% | %s\n",
			s.Code, compact(s.Volume), s.Multiple, s.PctChange, "ADL "+s.Interpretation())
	}
	return b.String()
}

func (n *Notifier) Send(ctx context.Context, text string) error {
	if !n.IsEnabled() {
		return fmt.Errorf("telegram: bot_token/chat_id not configured")
	}
	u := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.BotToken)
	form := url.Values{"chat_id": {n.ChatID}, "text": {text}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram: http %d", resp.StatusCode)
	}
	return nil
}
