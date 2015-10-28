package backend

// TODO: bypass mode in local backend

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/mission-liao/dingo/common"
	"github.com/mission-liao/dingo/meta"
)

//
// configuration
//

type _localConfig struct {
	Bypass_ bool `json:"Bypass"`
}

func (me *_localConfig) Bypass(yes bool) *_localConfig {
	me.Bypass_ = yes
	return me
}

func defaultLocalConfig() *_localConfig {
	return &_localConfig{
		Bypass_: true,
	}
}

type _local struct {
	cfg           *Config
	stores        *common.Routines
	to            chan []byte
	noJSON        chan meta.Report
	reporters     *common.Routines
	reportersLock sync.Mutex
	reports       chan meta.Report
	storeLock     sync.Mutex
	toCheck       []string
	unSent        []meta.Report
}

// factory
func newLocal(cfg *Config) (v *_local, err error) {
	v = &_local{
		cfg:       cfg,
		stores:    common.NewRoutines(),
		reporters: common.NewRoutines(),
		to:        make(chan []byte, 10),
		noJSON:    make(chan meta.Report, 10),
		reports:   make(chan meta.Report, 10),
		toCheck:   make([]string, 0, 10),
		unSent:    make([]meta.Report, 0, 10),
	}

	// Store -> Subscriber
	quit, wait := v.stores.New()
	go v._store_routine_(quit, wait)

	return
}

func (me *_local) _reporter_routine_(quit <-chan int, wait *sync.WaitGroup, reports <-chan meta.Report) {
	defer wait.Done()

	for {
		select {
		case _, _ = <-quit:
			goto cleanup
		case v, ok := <-reports:
			if !ok {
				// TODO:
				goto cleanup
			}

			if me.cfg.Local.Bypass_ {
				me.noJSON <- v
			} else {
				body, err := json.Marshal(v)
				if err != nil {
					// TODO: an error channel to reports errors
					break
				}

				// send to Store
				me.to <- body
			}
		}
	}
cleanup:
}

func (me *_local) _store_routine_(quit <-chan int, wait *sync.WaitGroup) {
	defer wait.Done()

	out := func(rep meta.Report) {
		me.storeLock.Lock()
		defer me.storeLock.Unlock()

		found := false
		for _, v := range me.toCheck {
			if v == rep.GetId() {
				found = true
				me.reports <- rep
				break
			}
		}

		if !found {
			me.unSent = append(me.unSent, rep)
		}
	}

	for {
		select {
		case _, _ = <-quit:
			goto cleanup
		case v, ok := <-me.to:
			if !ok {
				// TODO:
				goto cleanup
			}

			rep, err := meta.UnmarshalReport(v)
			if err != nil {
				// TODO:
				break
			}

			if rep == nil {
				break
			}

			out(rep)
		case v, ok := <-me.noJSON:
			if !ok {
				goto cleanup
			}

			out(v)
		}
	}
cleanup:
}

//
// common.Server interface
//

func (me *_local) Close() (err error) {
	me.stores.Close()

	err = me.Unbind()

	close(me.reports)
	close(me.to)
	close(me.noJSON)

	return
}

//
// Reporter
//

func (me *_local) Report(reports <-chan meta.Report) (err error) {
	me.reportersLock.Lock()
	defer me.reportersLock.Unlock()

	remain := me.cfg.Reporters_
	for ; remain > 0; remain-- {
		quit, wait := me.reporters.New()
		go me._reporter_routine_(quit, wait, reports)
	}

	if remain > 0 {
		err = errors.New(fmt.Sprintf("Still %v reporters uninitiated", remain))
	}
	return
}

func (me *_local) Unbind() (err error) {
	me.reportersLock.Lock()
	defer me.reportersLock.Unlock()

	me.reporters.Close()
	return
}

//
// Store
//

func (me *_local) Subscribe() (reports <-chan meta.Report, err error) {
	reports = me.reports
	return
}

func (me *_local) Poll(id meta.ID) (err error) {
	me.storeLock.Lock()
	defer me.storeLock.Unlock()

	for i := len(me.unSent) - 1; i >= 0; i-- {
		v := me.unSent[i]
		if v.GetId() == id.GetId() {
			me.reports <- v
			// delete this element
			me.unSent = append(me.unSent[:i], me.unSent[i+1:]...)
		}
	}

	found := false
	for _, v := range me.toCheck {
		if v == id.GetId() {
			found = true
		}
	}

	if !found {
		me.toCheck = append(me.toCheck, id.GetId())
	}

	return
}

func (me *_local) Done(id meta.ID) (err error) {
	me.storeLock.Lock()
	defer me.storeLock.Unlock()

	// clearing toCheck list
	for k, v := range me.toCheck {
		if v == id.GetId() {
			me.toCheck = append(me.toCheck[:k], me.toCheck[k+1:]...)
			break
		}
	}

	// clearing unSent
	for i := len(me.unSent) - 1; i >= 0; i-- {
		v := me.unSent[i]
		if v.GetId() == id.GetId() {
			// delete this element
			me.unSent = append(me.unSent[:i], me.unSent[i+1:]...)
		}
	}
	return
}
