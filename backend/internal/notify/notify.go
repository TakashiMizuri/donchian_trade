package notify

import "context"

type Notifier interface {
	Alert(ctx context.Context, level, kind, message string)
	Report(ctx context.Context, mail ReportMail)
}

type ReportMail struct {
	Date    string
	Caption string
	HTML    string
	PNG     []byte
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

func (m Multi) Report(ctx context.Context, mail ReportMail) {
	for _, n := range m.List {
		if n != nil {
			n.Report(ctx, mail)
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

func (f FuncNotifier) Report(_ context.Context, mail ReportMail) {
	if f.F != nil {
		f.F("info", "daily", mail.Caption+"\n"+mail.HTML)
	}
}

type Nop struct{}

func (Nop) Alert(context.Context, string, string, string) {}
func (Nop) Report(context.Context, ReportMail)            {}
