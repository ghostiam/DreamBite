package oscquery

import (
	"strconv"
)

type Access int

const (
	AccessNone      Access = 0
	AccessRead      Access = 1
	AccessWrite     Access = 2
	AccessReadWrite Access = AccessRead | AccessWrite
)

func (a Access) CanRead() bool {
	return a&AccessRead != 0
}

func (a Access) CanWrite() bool {
	return a&AccessWrite != 0
}

func (a Access) String() string {
	switch a {
	case AccessNone:
		return "None"
	case AccessRead:
		return "Read"
	case AccessWrite:
		return "Write"
	case AccessReadWrite:
		return "Read/Write"
	}

	return "Unknown(" + strconv.Itoa(int(a)) + ")"
}

type Type string

const (
	TypeInt    Type = "i"
	TypeFloat  Type = "f"
	TypeString Type = "s"
	TypeBool   Type = "T"
)

//nolint:tagliatelle
type HostInfo struct {
	Name         string          `json:"NAME,omitempty"`
	OscIP        string          `json:"OSC_IP,omitempty"`
	OscPort      int             `json:"OSC_PORT,omitempty"`
	OscTransport string          `json:"OSC_TRANSPORT,omitempty"`
	Extensions   map[string]bool `json:"EXTENSIONS"`
}
