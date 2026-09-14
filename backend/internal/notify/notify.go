package notify

import "context"

type Notifier interface {
	Alert(ctx context.Context, level, kind, message string)
}

type Multi struct {
	List []Notifier
}

func (m Multi) Alert(ctx context.Context, level, kind, message string) {
	for _, n := range m.List {
		if n != nil {
			n.Alert(ctx, level, kind, message)
		}
	}
}

type LogFunc func(level, kind, message string)

type FuncNotifier struct{ F LogFunc }

func (f FuncNotifier) Alert(_ context.Context, level, kind, message string) {
	if f.F != nil {
		f.F(level, kind, message)
	}
}

type Nop struct{}

func (Nop) Alert(context.Context, string, string, string) {}
