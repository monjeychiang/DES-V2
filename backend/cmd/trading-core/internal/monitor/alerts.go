package monitor

// AlertSink interface for pluggable alert delivery.
type AlertSink interface {
	Send(message string) error
}
