package data

import "time"

// LogDay coarsens t to the day: UTC midnight.
func LogDay(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// CoarseLogEntry returns e as an adapter stores it: the time coarsened to the
// day and no sequence (the adapter assigns one). Adapters compare entries in
// this form, so a replay of an entry stored before coarsening is a no-op.
func CoarseLogEntry(e LogEntry) LogEntry {
	e.At = LogDay(e.At)
	e.Seq = 0
	return e
}

// LogBefore reports whether log entry a lists before b, newest first: later
// day first, then higher Seq (written later), then, for entries stored before
// sequences, later full time, then ID descending. Every adapter lists the log
// in this order and builds its cursor from the same key.
func LogBefore(a, b LogEntry) bool {
	if da, db := LogDay(a.At), LogDay(b.At); !da.Equal(db) {
		return da.After(db)
	}
	if a.Seq != b.Seq {
		return a.Seq > b.Seq
	}
	if !a.At.Equal(b.At) {
		return a.At.After(b.At)
	}
	return a.ID > b.ID
}
