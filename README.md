# go-cfg-reloader

JSON config reloader

```go
import (
	"context"
	"time"

	reloader "github.com/ndmsystems/go-cfg-reloader"
)

type Config struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// Load reads and parses the config files once, synchronously. An error here
// means there's no usable config yet.
cr, err := reloader.New[Config](
	[]string{"config-default.json", "config-instance.json"},
	3*time.Second, // batches rapid successive file changes into one reload
)
if err != nil {
	// handle error
	os.Exit(1)
}

// the initial config is available right away
cfg := cr.Config()

// Subscribe registers a callback for subsequent reloads. Call before Start,
// so no update can be missed in between.
cr.Subscribe(func(oldCfg, newCfg Config) {
	// react to the change
})

// OnError is optional: observes errors from the background watcher. The
// reloader keeps serving the last good config either way.
cr.OnError(func(err error) {
	// log it, if you want
})

if err := cr.Start(context.Background()); err != nil {
	// handle error
}
```
