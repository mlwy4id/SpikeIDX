package domain

import "time"

func wib() *time.Location {
	if loc, err := time.LoadLocation("Asia/Jakarta"); err == nil {
		return loc
	}
	return time.FixedZone("WIB", 7*3600)
}

type TradingDate struct {
	t time.Time
}

func NewTradingDate(t time.Time) TradingDate {
	w := t.In(wib())
	return TradingDate{t: time.Date(w.Year(), w.Month(), w.Day(), 0, 0, 0, 0, wib())}
}

func ParseTradingDate(s string) (TradingDate, error) {
	t, err := time.ParseInLocation("2006-01-02", s, wib())
	if err != nil {
		return TradingDate{}, err
	}
	return TradingDate{t: t}, nil
}

func (d TradingDate) String() string { return d.t.Format("2006-01-02") }

func (d TradingDate) Time() time.Time { return d.t }

func (d TradingDate) Equal(o TradingDate) bool { return d.t.Equal(o.t) }
