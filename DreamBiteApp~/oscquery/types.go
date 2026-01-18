package oscquery

import (
	"errors"
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

type Endpoint[Handler any] struct {
	FullPath     string
	Access       Access
	Type         Type
	DefaultValue []any  `exhaustruct:"optional"`
	Description  string `exhaustruct:"optional"`
	Handler      Handler
}

func (ep *Endpoint[Handler]) Validate() error {
	if ep.FullPath == "" {
		return errors.New("path cannot be empty")
	}
	if ep.Access == AccessNone {
		return errors.New("access cannot be none")
	}
	if ep.Type == "" {
		return errors.New("type cannot be empty")
	}
	return nil
}

type HostInfo struct {
	Name string
	// OscIP is optional. Leave empty to use server IP.
	OscIP   string `exhaustruct:"optional"`
	OscPort int
	// OscTransport should be either "UDP" or "TCP". Default is UDP.
	OscTransport string `exhaustruct:"optional"`
}
