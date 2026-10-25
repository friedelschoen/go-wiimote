package driver

//go:generate morestringer -output stringer.go Backend

type Backend uint

const (
	BackendHID Backend = iota
	BackendKernel
)
