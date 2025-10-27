# Using slog.Record as a go error

Required code:

```go
package example

import "log/slog"
import "time"

type SlogError slog.Record

func (e SlogError) Error() string {
	return e.Message
}

func Error(msg string, args ...any) error {
	r := slog.NewRecord(time.Now(), slog.LevelError, msg, 0)
	return SlogError(r)
}

type MultiError []slog.Record

func (m MultiError) Error() string {
	return (m[0]).Message
}

```

Such errors work nicely with slog.Handler and any backend implementing this interface.

But they also provide the same ability to use structured output and attributes.

