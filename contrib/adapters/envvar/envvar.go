package envvar

import (
	"encoding"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/aatuh/envvar/v2"
	"github.com/aatuh/envvar/v2/loaders"

	"github.com/aatuh/api-toolkit/v2/ports"
)

// Adapter provides environment variable access using the envvar library.
type Adapter struct{}

// New creates a new envvar adapter that satisfies ports.EnvVar.
func New() ports.EnvVar { return &Adapter{} }

// LoadEnvFiles loads environment variables from files.
// Tries .env then /env/.env by default.
func (a *Adapter) LoadEnvFiles(paths []string) error {
	return loaders.LoadOnce(paths)
}

// MustLoadEnvFiles panics on errors when loading environment files.
func (a *Adapter) MustLoadEnvFiles(paths []string) {
	if err := a.LoadEnvFiles(paths); err != nil {
		panic(err)
	}
}

// Get returns the raw value and presence indicator.
func (a *Adapter) Get(key string) (string, bool) {
	v, ok := envvar.Get(key)
	return v, ok
}

// GetOr returns the value or default if not present.
func (a *Adapter) GetOr(key, def string) string {
	return envvar.GetOr(key, def)
}

// MustGet returns the value or panics if not present.
func (a *Adapter) MustGet(key string) string {
	return envvar.MustGet(key)
}

// GetBoolOr returns the value as boolean or default if not present.
func (a *Adapter) GetBoolOr(key string, def bool) bool {
	return envvar.GetBoolOr(key, def)
}

// MustGetBool returns the value as boolean or panics if not present.
func (a *Adapter) MustGetBool(key string) bool {
	return envvar.MustGetBool(key)
}

// GetIntOr returns the value as integer or default if not present.
func (a *Adapter) GetIntOr(key string, def int) int {
	v, ok := envvar.Get(key)
	if !ok {
		return def
	}
	if i, err := strconv.Atoi(v); err == nil {
		return i
	}
	return def
}

// MustGetInt returns the value as integer or panics if not present.
func (a *Adapter) MustGetInt(key string) int {
	return envvar.MustGetInt(key)
}

// GetInt64Or returns the value as int64 or default if not present.
func (a *Adapter) GetInt64Or(key string, def int64) int64 {
	v, ok := envvar.Get(key)
	if !ok {
		return def
	}
	if i, err := strconv.ParseInt(v, 10, 64); err == nil {
		return i
	}
	return def
}

// MustGetInt64 returns the value as int64 or panics if not present.
func (a *Adapter) MustGetInt64(key string) int64 {
	v := envvar.MustGet(key)
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		panic("environment variable " + key + " is not a valid int64: " + v)
	}
	return i
}

// GetUintOr returns the value as uint or default if not present.
func (a *Adapter) GetUintOr(key string, def uint) uint {
	v, ok := envvar.Get(key)
	if !ok {
		return def
	}
	if i, err := strconv.ParseUint(v, 10, 32); err == nil {
		return uint(i)
	}
	return def
}

// MustGetUint returns the value as uint or panics if not present.
func (a *Adapter) MustGetUint(key string) uint {
	v := envvar.MustGet(key)
	i, err := strconv.ParseUint(v, 10, 32)
	if err != nil {
		panic("environment variable " + key + " is not a valid uint: " + v)
	}
	return uint(i)
}

// GetUint64Or returns the value as uint64 or default if not present.
func (a *Adapter) GetUint64Or(key string, def uint64) uint64 {
	v, ok := envvar.Get(key)
	if !ok {
		return def
	}
	if i, err := strconv.ParseUint(v, 10, 64); err == nil {
		return i
	}
	return def
}

// MustGetUint64 returns the value as uint64 or panics if not present.
func (a *Adapter) MustGetUint64(key string) uint64 {
	v := envvar.MustGet(key)
	i, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		panic("environment variable " + key + " is not a valid uint64: " + v)
	}
	return i
}

// GetFloat64Or returns the value as float64 or default if not present.
func (a *Adapter) GetFloat64Or(key string, def float64) float64 {
	v, ok := envvar.Get(key)
	if !ok {
		return def
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	return def
}

// MustGetFloat64 returns the value as float64 or panics if not present.
func (a *Adapter) MustGetFloat64(key string) float64 {
	v := envvar.MustGet(key)
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		panic("environment variable " + key + " is not a valid float64: " + v)
	}
	return f
}

// GetDurationOr returns the value as duration or default if not present.
func (a *Adapter) GetDurationOr(key string, def time.Duration) time.Duration {
	v, ok := envvar.Get(key)
	if !ok {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

// MustGetDuration returns the value as duration or panics if not present.
func (a *Adapter) MustGetDuration(key string) time.Duration {
	v := envvar.MustGet(key)
	d, err := time.ParseDuration(v)
	if err != nil {
		panic("environment variable " + key + " is not a valid duration: " + v)
	}
	return d
}

// Bind populates a struct from environment variables.
// This is a simplified implementation that doesn't use struct tags.
func (a *Adapter) Bind(dst any) error {
	return bindStruct(dst, "")
}

// MustBind panics on binding errors.
func (a *Adapter) MustBind(dst any) {
	if err := a.Bind(dst); err != nil {
		panic(err)
	}
}

// BindWithPrefix binds with a prefix.
func (a *Adapter) BindWithPrefix(dst any, prefix string) error {
	return bindStruct(dst, prefix)
}

// MustBindWithPrefix panics on binding errors with prefix.
func (a *Adapter) MustBindWithPrefix(dst any, prefix string) {
	if err := a.BindWithPrefix(dst, prefix); err != nil {
		panic(err)
	}
}

// DumpRedacted returns environment with secrets redacted.
func (a *Adapter) DumpRedacted() map[string]string {
	env := os.Environ()
	out := make(map[string]string, len(env))
	for _, kv := range env {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		upper := strings.ToUpper(k)
		if strings.Contains(upper, "SECRET") ||
			strings.Contains(upper, "TOKEN") ||
			strings.Contains(upper, "PASSWORD") ||
			strings.HasSuffix(upper, "KEY") {
			out[k] = "***"
		} else {
			out[k] = v
		}
	}
	return out
}

var (
	errBindTarget = errors.New("bind target must be a non-nil pointer to struct")
)

func bindStruct(dst any, prefix string) error {
	if dst == nil {
		return errBindTarget
	}
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return errBindTarget
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return errBindTarget
	}
	rt := rv.Type()
	var errs []string
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := strings.TrimSpace(field.Tag.Get("env"))
		if name == "-" {
			continue
		}
		if name == "" {
			name = strings.TrimSpace(field.Tag.Get("envvar"))
		}
		if name == "" {
			name = toEnvKey(field.Name)
		}
		if prefix != "" {
			name = prefix + name
		}
		raw, ok := os.LookupEnv(name)
		if !ok {
			if def := field.Tag.Get("default"); def != "" {
				raw = def
				ok = true
			}
		}
		if !ok {
			continue
		}
		if err := setValue(rv.Field(i), raw); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("envvar bind: %s", strings.Join(errs, "; "))
	}
	return nil
}

func setValue(v reflect.Value, raw string) error {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return setValue(v.Elem(), raw)
	}
	if v.CanAddr() {
		if u, ok := v.Addr().Interface().(encoding.TextUnmarshaler); ok {
			return u.UnmarshalText([]byte(raw))
		}
	}
	//exhaustive:ignore reflect.Kind is open-ended and defaults to unsupported.
	switch v.Kind() {
	case reflect.String:
		v.SetString(raw)
		return nil
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		v.SetBool(b)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.Type() == reflect.TypeOf(time.Duration(0)) {
			d, err := time.ParseDuration(raw)
			if err != nil {
				return err
			}
			v.SetInt(int64(d))
			return nil
		}
		n, err := strconv.ParseInt(raw, 10, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetInt(n)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetUint(n)
		return nil
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(raw, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetFloat(f)
		return nil
	default:
		return fmt.Errorf("unsupported kind %s", v.Kind())
	}
}

func toEnvKey(name string) string {
	if name == "" {
		return ""
	}
	var b strings.Builder
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteByte(c)
			continue
		}
		if c >= 'a' && c <= 'z' {
			b.WriteByte(c - 32)
			continue
		}
		if c >= '0' && c <= '9' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('_')
	}
	return b.String()
}
