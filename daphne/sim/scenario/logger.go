package scenario

//go:generate mockgen -source logger.go -destination=logger_mock.go -package=scenario

// Logger is an interface for logging messages during scenario executions.
// TODO: replace this with proper logging interfaces facilitating the creation
// of nested loggers using a "With" function.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
}
