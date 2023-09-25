# README #
The log package is used by Go services to log messages to STDOUT.

### How To Use ###
To use the log package you first need to import the library into your project:
```
import "github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
```
To send a log message to STDOUT, you first need to create a Logger with desired Log Level and then you can send log messages (indicating the log level of the message using one of the various log function calls).
```
logger = log.InitLogger("SERVICE_NAME", configuration.LogLevel, nil) 

logger.Info("Something interesting")
logger.Infof("Starting %s %s ", internal.CoreDataServiceKey, edgex.Version)
logger.Errorf("Something bad happened: %s", err.Error())
```
Log messages can be logged as Info, Debug, Trace, Warn, or Error
