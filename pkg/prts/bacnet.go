//
// Copyright (C) 2026 IOTech Ltd
//

package prts

import "fmt"

// objectTypeMap maps BACnet object type IDs to human-readable names.
// Definitions sourced from https://github.com/bacnet-stack/bacnet-stack/blob/bacnet-stack-1.0/src/bacnet/bacenum.h#L1179-L1252
var objectTypeMap = map[int]string{
	0:  "Analog Input",
	1:  "Analog Output",
	2:  "Analog Value",
	3:  "Binary Input",
	4:  "Binary Output",
	5:  "Binary Value",
	6:  "Calendar",
	7:  "Command",
	8:  "Device",
	9:  "Event Enrollment",
	10: "File",
	11: "Group",
	12: "Loop",
	13: "Multi State Input",
	14: "Multi State Output",
	15: "Notification Class",
	16: "Program",
	17: "Schedule",
	18: "Averaging",
	19: "Multi State Value",
	20: "Trendlog",
	21: "Life Safety Point",
	22: "Life Safety Zone",
	23: "Accumulator",
	24: "Pulse Converter",
	25: "Event Log",
	26: "Global Group",
	27: "Trend Log Multiple",
	28: "Load Control",
	29: "Structured View",
	30: "Access Door",
	31: "Timer",
	32: "Access Credential",
	33: "Access Point",
	34: "Access Rights",
	35: "Access User",
	36: "Access Zone",
	37: "Credential Data Input",
	38: "Network Security",
	39: "Bitstring Value",
	40: "Characterstring Value",
	41: "Date Pattern Value",
	42: "Date Value",
	43: "Datetime Pattern Value",
	44: "Datetime Value",
	45: "Integer Value",
	46: "Large Analog Value",
	47: "Octetstring Value",
	48: "Positive Integer Value",
	49: "Time Pattern Value",
	50: "Time Value",
	51: "Notification Forwarder",
	52: "Alert Enrollment",
	53: "Channel",
	54: "Lighting Output",
	55: "Binary Lighting Output",
	56: "Network Port",
	57: "Elevator Group",
	58: "Escalator",
	59: "Lift",
	60: "Staging",
}

// BACnet object type range constants.
// Enumerated values 0-127 are reserved for definition by ASHRAE.
// Enumerated values 128-1023 may be used by others subject to the procedures and constraints described in Clause 23.
// Refer: https://github.com/bacnet-stack/bacnet-stack/blob/bacnet-stack-1.0/src/bacnet/bacenum.h#L1242-L1247
const (
	ObjectProprietaryMin = 128
	MaxObjectType        = 1024
	ObjectTypeNone       = 65535 // 0xFFFFu — sentinel value defined by the BACnet spec
	ObjectTypeNoneString = "None"
)

// GetBACnetObjectTypeName returns the human-readable string for a BACnet object type ID.
func GetBACnetObjectTypeName(objectType int) (string, error) {
	if objectType < 0 {
		return "", fmt.Errorf("object type %d is invalid", objectType)
	}
	if objectType == ObjectTypeNone {
		return ObjectTypeNoneString, nil
	}
	if objectType >= MaxObjectType {
		return "", fmt.Errorf("object type %d exceeds max BACnet object limit", objectType)
	}

	if objectTypeStr, ok := objectTypeMap[objectType]; ok {
		return objectTypeStr, nil
	}

	// Values 128-1023 are proprietary, defined by vendors per Clause 23
	if objectType >= ObjectProprietaryMin {
		return fmt.Sprintf("Proprietary Type (%d)", objectType), nil
	}

	// Values 0-127 not in the map are reserved by ASHRAE but not yet defined
	return fmt.Sprintf("Reserved Type (%d)", objectType), nil
}
