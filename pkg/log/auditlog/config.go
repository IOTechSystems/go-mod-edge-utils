package auditlog

type Configuration struct {
	// StorageDir is the directory to write logs to.
	StorageDir string

	// Filename is the file to write logs to.  Backup log files will be retained
	// in the same directory.
	FileName string

	// MaxSize is the maximum size in megabytes of the log file before it gets
	// rotated. It defaults to 100 megabytes.
	MaxSize int

	// MaxAge is the maximum number of days to retain old log files based on the
	// timestamp encoded in their filename.  Note that a day is defined as 24
	// hours and may not exactly correspond to calendar days due to daylight
	// savings, leap seconds, etc. The default is not to remove old log files
	// based on age.
	MaxAge int

	// MaxBackups is the maximum number of old log files to retain.  The default
	// is to retain all old log files (though MaxAge may still cause them to get
	// deleted.)
	MaxBackups int
}

var defaultConfig = Configuration{
	StorageDir: "/tmp/log/audit",
	FileName:   "audit.log",
	MaxSize:    100,
	MaxAge:     28,
	MaxBackups: 3,
}

func (c *Configuration) setDefault() {
	if c.StorageDir == "" {
		c.StorageDir = defaultConfig.StorageDir
	}
	if c.FileName == "" {
		c.FileName = defaultConfig.FileName
	}
	if c.MaxSize == 0 {
		c.MaxSize = defaultConfig.MaxSize
	}
	if c.MaxAge == 0 {
		c.MaxAge = defaultConfig.MaxAge
	}
	if c.MaxBackups == 0 {
		c.MaxBackups = defaultConfig.MaxBackups
	}
}
