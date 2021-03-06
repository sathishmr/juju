// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package engine

import (
	"github.com/juju/errors"
	worker "gopkg.in/juju/worker.v1"
	tomb "gopkg.in/tomb.v1"
)

// Flag represents a single boolean used to determine whether a given
// manifold worker should run.
type Flag interface {

	// Check returns the flag's value. Check calls must *always* return
	// the same value for a given instatiation of the type implementing
	// Flag.
	Check() bool
}

// FlagOutput will expose, as a Flag, any worker that implements Flag.
func FlagOutput(in worker.Worker, out interface{}) error {
	inFlag, ok := in.(Flag)
	if !ok {
		return errors.Errorf("expected in to implement Flag; got a %T", in)
	}
	outFlag, ok := out.(*Flag)
	if !ok {
		return errors.Errorf("expected out to be a *Flag; got a %T", out)
	}
	*outFlag = inFlag
	return nil
}

type staticFlagWorker struct {
	tomb  tomb.Tomb
	value bool
}

// NewStaticFlagWorker returns a new Worker that implements Flag,
// whose Check method always returns the specified value.
func NewStaticFlagWorker(value bool) worker.Worker {
	w := &staticFlagWorker{value: value}
	go func() {
		defer w.tomb.Done()
		defer w.tomb.Kill(tomb.ErrDying)
		<-w.tomb.Dying()
	}()
	return w
}

// Check is part of the Flag interface.
func (w *staticFlagWorker) Check() bool {
	return w.value
}

// Kill is part of the Worker interface.
func (w *staticFlagWorker) Kill() {
	w.tomb.Kill(nil)
}

// Wait is part of the Worker interface.
func (w *staticFlagWorker) Wait() error {
	return w.tomb.Wait()
}
