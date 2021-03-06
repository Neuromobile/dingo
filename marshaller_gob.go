package dingo

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"reflect"
)

/*GobMarshaller is a marshaller implemented via gob encoding.
Note: this Marshaller can work with both GenericInvoker and LazyInvoker.
*/
type GobMarshaller struct{}

func (ms *GobMarshaller) Prepare(name string, fn interface{}) (err error) {
	var (
		fT reflect.Type
	)
	// Gob needs to register type before encode/decode
	if fT = reflect.TypeOf(fn); fT.Kind() != reflect.Func {
		err = fmt.Errorf("fn is not a function but %v", fn)
		return
	}

	reg := func(v reflect.Value) (err error) {
		if !v.CanInterface() {
			err = fmt.Errorf("Can't convert to value in input of %v for name:%v", fn, name)
			return
		}

		gob.Register(v.Interface())
		return
	}

	for i := 0; i < fT.NumIn(); i++ {
		// create a zero value of the type of parameters
		if err = reg(reflect.Zero(fT.In(i))); err != nil {
			return
		}
	}

	for i := 0; i < fT.NumOut(); i++ {
		if err = reg(reflect.Zero(fT.Out(i))); err != nil {
			return
		}
	}

	return
}

func (ms *GobMarshaller) EncodeTask(fn interface{}, task *Task) (b []byte, err error) {
	if task == nil {
		err = errors.New("nil is bad for Gob")
		return
	}

	// reset registry
	task.H.Reset()

	// encode header
	var bHead []byte
	if bHead, err = task.H.Flush(0); err != nil {
		return
	}

	// encode payload
	var buff = bytes.NewBuffer(bHead)
	if err = gob.NewEncoder(buff).Encode(task.P); err == nil {
		b = buff.Bytes()
	}
	return
}

func (ms *GobMarshaller) DecodeTask(h *Header, fn interface{}, b []byte) (task *Task, err error) {
	// decode header
	if h == nil {
		if h, err = DecodeHeader(b); err != nil {
			return
		}
	}

	// clean registry when leaving
	defer func() {
		if h != nil {
			h.Reset()
		}
	}()

	// decode payload
	var payload *TaskPayload
	if err = gob.NewDecoder(bytes.NewBuffer(b[h.Length():])).Decode(&payload); err == nil {
		task = &Task{
			H: h,
			P: payload,
		}
	}
	return
}

func (ms *GobMarshaller) EncodeReport(fn interface{}, report *Report) (b []byte, err error) {
	if report == nil {
		err = errors.New("nil is bad for Gob")
		return
	}

	// reset registry
	report.H.Reset()

	// encode header
	var bHead []byte
	if bHead, err = report.H.Flush(0); err != nil {
		return
	}

	// encode payload
	var buff = bytes.NewBuffer(bHead)
	if err = gob.NewEncoder(buff).Encode(report.P); err == nil {
		b = buff.Bytes()
	}
	return
}

func (ms *GobMarshaller) DecodeReport(h *Header, fn interface{}, b []byte) (report *Report, err error) {
	// decode header
	if h == nil {
		if h, err = DecodeHeader(b); err != nil {
			return
		}
	}

	// clean registry when leaving
	defer func() {
		if h != nil {
			h.Reset()
		}
	}()

	// decode payload
	var payload *ReportPayload
	if err = gob.NewDecoder(bytes.NewBuffer(b[h.Length():])).Decode(&payload); err == nil {
		report = &Report{
			H: h,
			P: payload,
		}
	}
	return
}
