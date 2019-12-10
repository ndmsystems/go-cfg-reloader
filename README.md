# go-cfg-reloader
JSON config reloader

```go
// app config struct
type App struct {
	Host    string
	Port    string
	Key     string
	Pass    string
}

basePath := "/usr/local/app/settings/"

reloader := reloader.New(
    []string{
        filepath.Join(basePath, "app-default.json"),
    },
    func(err error) { fmt.Println(err) },
)

// config handler called when config reloaded
cfgHandler := func(key string, data json.RawMessage) {
    obj := new(App)
    if err := json.Unmarshal(data, &obj); err != nil {
        // handle error
    }
    // got config data under "app" json key
}

reloader.KeyAdd("app", cfgHandler)
reloader.Start()
```
