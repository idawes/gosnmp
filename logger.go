package gosnmp

// LoggerInterface is the interface that snmp-go expects to be able to use to produce debug logging.
// It matches exactly with the interface presented by SeeLog (github.com/cihub/seelog), but should be very simple to adapt to
// pretty much any reasonable logging infrastructure.
type Logger interface {

	// Tracef formats message according to format specifier
	// and writes to log with level = Trace.
	Tracef(format string, params ...interface{})

	// Debugf formats message according to format specifier
	// and writes to log with level = Debug.
	Debugf(format string, params ...interface{})

	// Infof formats message according to format specifier
	// and writes to log with level = Info.
	Infof(format string, params ...interface{})

	// Warnf formats message according to format specifier
	// and writes to log with level = Warn.
	Warnf(format string, params ...interface{}) error

	// Errorf formats message according to format specifier
	// and writes to log with level = Error.
	Errorf(format string, params ...interface{}) error

	// Criticalf formats message according to format specifier
	// and writes to log with level = Critical.
	Criticalf(format string, params ...interface{}) error

	// Trace formats message using the default formats for its operands
	// and writes to log with level = Trace
	Trace(v ...interface{})

	// Debug formats message using the default formats for its operands
	// and writes to log with level = Debug
	Debug(v ...interface{})

	// Info formats message using the default formats for its operands
	// and writes to log with level = Info
	Info(v ...interface{})

	// Warn formats message using the default formats for its operands
	// and writes to log with level = Warn
	Warn(v ...interface{}) error

	// Error formats message using the default formats for its operands
	// and writes to log with level = Error
	Error(v ...interface{}) error

	// Critical formats message using the default formats for its operands
	// and writes to log with level = Critical
	Critical(v ...interface{}) error
}
