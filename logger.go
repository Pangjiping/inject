package inject

// Logger allows for simple logging as inject traverses and populates the
// object graph.
type Logger interface {
	Debugf(format string, v ...interface{})
}
