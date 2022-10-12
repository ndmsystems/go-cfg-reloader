package reloader

import (
	"container/list"
	"sync"
	"time"

	"github.com/ndmsystems/go-cfg-reloader/api"
)

// eventDispatcher ...
type eventDispatcher struct {
	sync.Mutex
	ch    chan api.Event
	done  chan struct{}
	queue *list.List
}

func newEventDispatcher() *eventDispatcher {
	return &eventDispatcher{
		queue: list.New(),
		done:  make(chan struct{}),
	}
}

// getEventsChan ...
func (d *eventDispatcher) getEventsChan() <-chan api.Event {
	if d.ch != nil {
		return d.ch
	}

	d.ch = make(chan api.Event)

	go func() {
		for {
			select {
			case <-d.done:
				return
			default:
				if e, ok := d.pop(); ok {
					d.ch <- e
					continue
				}
				time.Sleep(200 * time.Millisecond)
			}
		}
	}()

	return d.ch
}

// push ..
func (d *eventDispatcher) push(reason string) {
	if d.ch == nil {
		return
	}

	d.Lock()
	defer d.Unlock()
	d.queue.PushBack(api.Event{
		Time:   time.Now(),
		Reason: reason,
	})
}

// pop ...
func (d *eventDispatcher) pop() (api.Event, bool) {
	d.Lock()
	defer d.Unlock()
	el := d.queue.Front()
	if el != nil {
		event := el.Value.(api.Event)
		d.queue.Remove(el)
		return event, true
	}
	return api.Event{}, false
}

// stop ...
func (d *eventDispatcher) stop() {
	close(d.done)
}
