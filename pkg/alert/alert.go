package alert

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/manzanit0/weathry/pkg/tgram"
)

type Notifier interface {
	Msg(ctx context.Context, msg string, args ...interface{}) error
	Recover(ctx context.Context)
}

type telegramNotifier struct {
	t              tgram.Client
	receiverChatID int64
}

func NewTelegramNotifier(t tgram.Client, receiverChatID int64) *telegramNotifier {
	return &telegramNotifier{t: t, receiverChatID: receiverChatID}
}

func (n *telegramNotifier) Msg(_ context.Context, msg string, args ...interface{}) error {
	return n.t.SendMessage(tgram.SendMessageRequest{
		ChatID:    n.receiverChatID,
		Text:      fmt.Sprintf(msg, args...),
		ParseMode: tgram.ParseModeMarkdownV1,
	})
}

func (n *telegramNotifier) Recover(_ context.Context) {
	if r := recover(); r != nil {
		callstack := getCallstack()
		n.t.SendMessage(tgram.SendMessageRequest{
			ParseMode: tgram.ParseModeHTML,
			ChatID:    n.receiverChatID,
			Text: fmt.Sprintf(`<b>Recovered from panic: %v</b>
<code>%s</code>`, r, callstack),
		})
	}
}

func getCallstack() string {
	pcs := make([]uintptr, 20)
	depth := runtime.Callers(3, pcs)
	frames := runtime.CallersFrames(pcs[:depth])

	var sb strings.Builder
	for f, more := frames.Next(); more; f, more = frames.Next() {
		sb.WriteString(fmt.Sprintf("%s: %d\n", f.Function, f.Line))
	}

	return sb.String()
}
