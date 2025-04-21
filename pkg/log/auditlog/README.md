# README #
The log/auditlog package is used by Go services to log audit messages to file.

If the file is inaccessible for writing or encounters unexpected errors, the audit messages will be directed to STDOUT instead.

### How To Use ###
To use the log package you first need to import the library into your project:
```
import "github.com/IOTechSystems/go-mod-edge-utils/pkg/log/auditlog"
```
To send an audit message to a file, you first need to create a Logger with `service name`, desired `coverage level`, desired `io.Writer` (write to file as default), `configurations` about log rotation, and then you can send audit messages (indicating the coverage level of the message using one of the various log function calls).

Audit messages can be logged as `Base`, `Advanced`, or `Full`.
```
logger := auditlog.InitLogger("SERVICE_NAME", "FULL", nil, auditlog.Configuration{})
	details := map[string]any{
		"key1": "value1",
		"key2": "value2",
	}

logger.LogFull(auditlog.SeverityNormal, "Admin", "auditlog.ActionTypeLogin", "description", nil)
logger.LogAdvanced(auditlog.SeverityNormal, "Admin", auditlog.ActionTypeLogin, "description", details)
logger.LogBase(auditlog.SeverityCritical, "Admin", auditlog.ActionTypeDelete, "description", details)
```

An audit message is composed of the following elements.

#### Severity

The supported severity levels are `Normal`, `Warning`, and `Critical`.

#### Actor
An `actor` is the entity that initiated the action that is being audited.

#### Action Type
There are various `action types` that can be specified. The action type is the type of action that is being audited.

#### Description
The `description` is a string that describes the action that is being audited.

#### Details
The `details` is a map of key-value pairs that provide additional information about the action that is being audited.