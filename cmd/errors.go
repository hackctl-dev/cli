package cmd

type silentError struct {
	err error
}

func (e silentError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e silentError) Unwrap() error {
	return e.err
}

func (e silentError) Silent() bool {
	return true
}

func silent(err error) error {
	if err == nil {
		return nil
	}
	return silentError{err: err}
}
