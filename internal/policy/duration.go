package policy

import "time"

// Duration is a time.Duration that marshals to and from a human string such as
// "20m" or "1h30m" in TOML, instead of a raw nanosecond count.
type Duration struct {
	time.Duration
}

// UnmarshalText implements encoding.TextUnmarshaler for TOML decoding.
func (d *Duration) UnmarshalText(text []byte) error {
	s := string(text)
	if s == "" || s == "0" {
		d.Duration = 0
		return nil
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = parsed
	return nil
}

// MarshalText implements encoding.TextMarshaler for TOML encoding.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// String renders the duration, using "0" for the unlimited (zero) case.
func (d Duration) String() string {
	if d.Duration == 0 {
		return "0"
	}
	return d.Duration.String()
}
