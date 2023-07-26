package reloader_test

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"testing"
	"time"

	reloader "github.com/ndmsystems/go-cfg-reloader"
	"github.com/stretchr/testify/require"
)

const fileDir = "testdata"

type logger struct {
}

func (l *logger) Info(vals ...interface{}) {
	log.Println(vals...)
}
func (l *logger) Error(vals ...interface{}) {
	log.Println(vals...)
}
func TestReloader(t *testing.T) {
	r := require.New(t)
	os.RemoveAll(fileDir)
	os.Mkdir(fileDir, 0o775)
	defer os.RemoveAll(fileDir)

	writeFile(fileDir+"/"+"cfg1.json", `{"x":1, "y":{"a":2}, "z":[3, 4], "thrash": 2222}`)
	writeFile(fileDir+"/"+"ignored.json", `{"x":2, "y":{"a":3, "b": 5}, "z":[5, 6]}`)

	cr := reloader.New([]string{fileDir + "/" + "cfg1.json", fileDir + "/" + "cfg2.json"}, 500*time.Millisecond, &logger{})
	type TSI struct {
		A int `json:"a"`
		B int `json:"b"`
	}
	type TS struct {
		X int   `json:"x"`
		Y TSI   `json:"y"`
		Z []int `json:"z"`
	}
	s := TS{}
	xChangedCount := 0
	// setup callback on all fields
	cr.KeyAdd("x", func(key string, data json.RawMessage) {
		s.X = 0
		if len(data) == 0 {
			return
		}
		r.NoError(json.Unmarshal(data, &s.X))
	})
	cr.KeyAdd("x", func(key string, data json.RawMessage) {
		xChangedCount++
	}) // two cbs allowed
	cr.KeyAdd("y", func(key string, data json.RawMessage) {
		s.Y = TSI{}
		if len(data) == 0 {
			return
		}
		r.NoError(json.Unmarshal(data, &s.Y))
	})
	cr.KeyAdd("z", func(key string, data json.RawMessage) {
		s.Z = nil
		if len(data) == 0 {
			return
		}
		r.NoError(json.Unmarshal(data, &s.Z))
	})
	r.NoError(cr.Start(context.Background()))
	// check all parsed on start
	r.Equal(TS{
		X: 1,
		Y: TSI{
			A: 2,
			B: 0,
		},
		Z: []int{3, 4},
	}, s)

	xChangedCount = 0
	writeFile(fileDir+"/"+"ignored.json", `{"x":3, "y":{"a":3}, "z":[5, 6]}`)
	time.Sleep(time.Second * 1)
	// nothing changed if ignored file changed
	r.Equal(0, xChangedCount)

	writeFile(fileDir+"/"+"cfg1.json", `{"x":1, "y":{"a":2}, "z":[3, 4], "thrash": 2222}`)
	time.Sleep(time.Second * 1)
	// field x not changed so no calls
	r.Equal(0, xChangedCount)

	writeFile(fileDir+"/"+"cfg1.json", `{"x":2, "y":{"a":2}, "z":[3, 4], "thrash": 2222}`)
	time.Sleep(time.Second * 1)
	// field x changed
	r.Equal(1, xChangedCount)
	r.Equal(TS{
		X: 2,
		Y: TSI{
			A: 2,
			B: 0,
		},
		Z: []int{3, 4},
	}, s)

	// test batching
	// do many operations and last returs initial value(so notihing happens)
	xChangedCount = 0
	writeFile(fileDir+"/"+"cfg1.json", `{"x":1, "y":{"a":2}, "z":[3, 4], "thrash": 2222}`)
	writeFile(fileDir+"/"+"cfg1.json", `{"x":3, "y":{"a":2}, "z":[3, 4], "thrash": 2222}`)
	writeFile(fileDir+"/"+"cfg1.json", `{"x":4, "y":{"a":2}, "z":[3, 4], "thrash": 2222}`)
	writeFile(fileDir+"/"+"cfg1.json", `{"x":2, "y":{"a":2}, "z":[3, 4], "thrash": 2222}`)
	time.Sleep(1 * time.Second)
	r.Equal(0, xChangedCount)
	r.Equal(TS{
		X: 2,
		Y: TSI{
			A: 2,
			B: 0,
		},
		Z: []int{3, 4},
	}, s)

	// adding new notifying file and check priorites and array merging
	writeFile(fileDir+"/"+"cfg2.json", `{"x":3, "y":{"b":4}, "z":[5,6]}`)
	time.Sleep(1 * time.Second)
	r.Equal(TS{
		X: 3,
		Y: TSI{
			A: 2,
			B: 4,
		},
		Z: []int{3, 4, 5, 6},
	}, s)
	os.Remove(fileDir + "/" + "cfg2.json")
	time.Sleep(1 * time.Second)
	r.Equal(TS{
		X: 2,
		Y: TSI{
			A: 2,
			B: 0,
		},
		Z: []int{3, 4},
	}, s) // no cfg2 so it like cfg1

	os.Rename(fileDir+"/"+"cfg1.json", fileDir+"/"+"thrash.json") // renaming file to some ignored name
	time.Sleep(1 * time.Second)
	// if all cfg files removed then it will not change it's value
	r.Equal(TS{
		X: 0,
		Y: TSI{
			A: 0,
			B: 0,
		},
		Z: nil,
	}, s)
	// check force reload and reload time
	os.Rename(fileDir+"/"+"thrash.json", fileDir+"/"+"cfg1.json")
	cr.ForceReload()
	r.Less(time.Since(cr.ReloadTime()), time.Millisecond)

}

func writeFile(fileName, content string) {
	err := os.WriteFile(fileName, []byte(content), 0o664)
	if err != nil {
		panic(err)
	}
}
