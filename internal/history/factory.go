package history

import "fmt"

// Driver names a Store backend that New knows how to construct.
type Driver string

const (
	DriverRedis Driver = "redis"
)

// Config holds the parameters needed to construct a Store via New.
type Config struct {
	Driver Driver
	Addr   string
	Cap    int
}

// New constructs the Store backend named by cfg.Driver.
func New(cfg Config) (Store, error) {
	switch cfg.Driver {
	case DriverRedis:
		return NewRedisStoreFromAddr(cfg.Addr, cfg.Cap)
	default:
		return nil, fmt.Errorf("history: unsupported driver %q", cfg.Driver)
	}
}
