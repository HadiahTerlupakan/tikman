package services

import (
	"errors"
	"strings"
)

// The names a failure is reported under on a report with both destinations.
const (
	destinationWhatsApp = "WhatsApp"
	destinationTelegram = "Telegram"
)

// sendFailure is one destination's failure; a blank destination is one that
// failed the whole send, such as LibreNMS.
type sendFailure struct {
	destination string
	err         error
}

// sendFailures are what a send failed on, and where it got through. named
// prefixes each failure with its destination, which only a report with both
// needs: on one with a single destination the page already says where the
// report goes.
type sendFailures struct {
	named     bool
	delivered []string
	failed    []sendFailure
}

func (f *sendFailures) add(destination string, err error) {
	f.failed = append(f.failed, sendFailure{destination: destination, err: err})
}

// record notes a send's outcome at one destination.
func (f *sendFailures) record(destination string, err error) {
	if err != nil {
		f.add(destination, err)
		return
	}
	f.delivered = append(f.delivered, destination)
}

// Error leads with what did get through. A half-sent report read as a plain
// failure invites a retry, and a manual retry posts again to the destination
// that already has it.
func (f sendFailures) Error() string {
	msgs := make([]string, len(f.failed))
	for i, failure := range f.failed {
		msgs[i] = f.describe(failure)
	}
	joined := strings.Join(msgs, "; ")
	if len(f.delivered) == 0 {
		return joined
	}
	return "Terkirim ke " + strings.Join(f.delivered, " dan ") + "; " + joined
}

func (f sendFailures) describe(failure sendFailure) string {
	var uncertain *telegramUncertainError
	switch {
	case failure.destination == "" || errors.As(failure.err, &uncertain):
		return failure.err.Error()
	case len(f.delivered) > 0:
		return failure.destination + " gagal: " + failure.err.Error()
	case f.named:
		return failure.destination + ": " + failure.err.Error()
	}
	return failure.err.Error()
}

func (f sendFailures) Unwrap() []error {
	errs := make([]error, len(f.failed))
	for i, failure := range f.failed {
		errs[i] = failure.err
	}
	return errs
}

// onlyUncertain says whether every failure is a send that may have arrived,
// which leaves every destination counted for the day.
func (f sendFailures) onlyUncertain() bool {
	for _, failure := range f.failed {
		var uncertain *telegramUncertainError
		if !errors.As(failure.err, &uncertain) {
			return false
		}
	}
	return len(f.failed) > 0
}

// orNil keeps an empty list from reading as a failure through the error interface.
func (f sendFailures) orNil() error {
	if len(f.failed) == 0 {
		return nil
	}
	return f
}
