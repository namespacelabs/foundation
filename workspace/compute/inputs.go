// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package compute

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fntypes"
)

const cacheVersion = 1 // Allow for global cache invalidation.

func Inputs() *In { return &In{} }

type In struct {
	ins         []keyValue
	marshallers []keyMarshal
	serial      int64
	named       rawComputable // Set by Named(). If set, short-circuits node computation and waits for this input.
}

type keyValue struct {
	Name         string
	Value        interface{}
	Undetermined bool
}

type keyMarshal struct {
	Name    string
	Marshal func(context.Context, io.Writer) error
}

func (in *In) Computable(key string, c rawComputable) *In {
	in.ins = append(in.ins, keyValue{Name: key, Value: c})
	return in
}

func (in *In) Str(key string, str string) *In {
	in.ins = append(in.ins, keyValue{Name: key, Value: str})
	return in
}

func (in *In) StrMap(key string, m map[string]string) *In {
	var kvs []keyValue
	for k, v := range m {
		kvs = append(kvs, keyValue{Name: k, Value: v})
	}
	// Make it stable
	sort.Slice(kvs, func(i, j int) bool { return strings.Compare(kvs[i].Name, kvs[j].Name) < 0 })
	return in.JSON(key, kvs)
}

func (in *In) Stringer(key string, str fmt.Stringer) *In {
	in.ins = append(in.ins, keyValue{Name: key, Value: str.String()})
	return in
}

func (in *In) Strs(key string, strs []string) *In {
	in.ins = append(in.ins, keyValue{Name: key, Value: strs})
	return in
}

func (in *In) JSON(key string, json interface{}) *In {
	in.ins = append(in.ins, keyValue{Name: key, Value: json})
	return in
}

func (in *In) Proto(key string, msg proto.Message) *In {
	in.ins = append(in.ins, keyValue{Name: key, Value: msg})
	return in
}

func (in *In) Bool(key string, v bool) *In {
	in.ins = append(in.ins, keyValue{Name: key, Value: v})
	return in
}

func (in *In) CacheRev(v int) *In {
	in.ins = append(in.ins, keyValue{Name: "fn.cache-rev", Value: v})
	return in
}

// An unusable input (marking the corresponding Computable having non-computable inputs).
// We accept a variable to help with code search; but it is otherwise unused.
func (in *In) Indigestible(key string, value interface{}) *In {
	in.ins = append(in.ins, keyValue{Name: key, Value: value, Undetermined: true})
	return in
}

func (in *In) Marshal(key string, marshaller func(context.Context, io.Writer) error) *In {
	in.marshallers = append(in.marshallers, keyMarshal{key, marshaller})
	return in
}

func (in *In) Version(serial int64) *In {
	in.serial = serial
	return in
}

func (in *In) Digest(key string, d Digestible) *In {
	in.marshallers = append(in.marshallers, keyMarshal{key, func(ctx context.Context, w io.Writer) error {
		digest, err := d.ComputeDigest(ctx)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(w, digest.String())
		return err
	}})
	return in
}

type keyDigest struct {
	Name   string
	Digest string
	Set    bool // If not set, the input was not deterministic. It will have to be then set via Finalize().
}

type computedInputs struct {
	serial           int64 // A type-provided digest function version. Bump it with `Inputs().Version()` if the cache function changes.
	pkgPath          string
	typeName         string
	digests          []keyDigest
	computable       map[string]rawComputable
	nonDeterministic bool // Even waiting for dependencies won't really lead to a deterministic digest.

	Digest            fntypes.Digest // Only set if all inputs are known recursively over all dependencies.
	PostComputeDigest fntypes.Digest // Only set after `Finalize`, and if all values were resolved.
}

func (c *computedInputs) Finalize(resolved map[string]ResultWithTimestamp[any]) error {
	if c.nonDeterministic {
		return nil
	}

	var computedInputs []keyDigest
	for _, kv := range c.digests {
		if !kv.Set {
			v, has := resolved[kv.Name]
			if !has {
				return fnerrors.InternalError("%s: computed value is missing", kv.Name)
			}

			if !v.Digest.IsSet() || v.NonDeterministic {
				// No output digest possible.
				return nil
			}

			computedInputs = append(computedInputs, keyDigest{Name: kv.Name, Digest: v.Digest.String(), Set: true})
		} else {
			computedInputs = append(computedInputs, kv)
		}
	}

	var err error
	c.PostComputeDigest, err = digestWithInputs(c.pkgPath, c.typeName, c.serial, computedInputs)
	return err
}

func digestWithInputs(pkgPath, typeName string, serial int64, inputs []keyDigest) (fntypes.Digest, error) {
	h := sha256.New()

	if _, err := fmt.Fprintf(h, "$V:%d\nPkgPath:%s\nType:%s\nVersion:%d\nInputs{\n", cacheVersion, pkgPath, typeName, serial); err != nil {
		return fntypes.Digest{}, err
	}

	for _, kv := range inputs {
		if _, err := fmt.Fprintf(h, "%s:%s\n", kv.Name, kv.Digest); err != nil {
			return fntypes.Digest{}, err
		}
	}

	if _, err := fmt.Fprint(h, "}\n"); err != nil {
		return fntypes.Digest{}, err
	}

	return fntypes.FromHash("sha256", h), nil
}

func (in *In) computeDigest(ctx context.Context, c rawComputable, processComputable bool) (*computedInputs, error) {
	typ := reflect.TypeOf(c)
	res := &computedInputs{
		serial:   in.serial,
		pkgPath:  typ.PkgPath(),
		typeName: typ.String(),
	}

	if processComputable {
		res.computable = map[string]rawComputable{}
	}

	unsetCount := 0
	for _, kv := range in.ins {
		depc, isComputable := kv.Value.(rawComputable)
		if processComputable && isComputable {
			res.computable[kv.Name] = depc
		}

		if kv.Undetermined {
			res.nonDeterministic = true
			continue
		}

		if !isComputable {
			if d, err := fntypes.DigestOf(kv.Value); err != nil {
				return nil, fnerrors.InternalError("%s: failed to compute digest: %w", kv.Name, err)
			} else {
				res.digests = append(res.digests, keyDigest{Name: kv.Name, Digest: d.String(), Set: true})
			}
			continue
		}

		if res.nonDeterministic {
			continue // No point computing more digests.
		}

		opts := depc.prepareCompute(depc)

		// If the dependency can't be loaded from cache, don't bother even checking if it has
		// a computed input digest.
		if !opts.CanCache() {
			res.digests = append(res.digests, keyDigest{Name: kv.Name})
			unsetCount++
			continue
		}

		// XXX check for cycles.
		computed, err := depc.Inputs().computeDigest(ctx, depc, false)
		if err != nil {
			return nil, fnerrors.InternalError("%s: failed with: %w", kv.Name, err)
		}

		kv := keyDigest{Name: kv.Name}

		// The dependency's input digest could not be fully calculated because one of
		// the inputs is not available, i.e. it needs to be computed.
		if computed.Digest.IsSet() {
			kv.Digest = computed.Digest.String()
			kv.Set = true
		} else {
			unsetCount++
		}

		// We serialize a digest of the inputs rather than the inputs themselves as we have
		// logic in digest.With() that aims for determinism.
		res.digests = append(res.digests, kv)
	}

	if res.nonDeterministic {
		return res, nil
	}

	for _, m := range in.marshallers {
		h := sha256.New()
		if err := m.Marshal(ctx, h); err != nil {
			return nil, fnerrors.InternalError("%s: marshaller failed with: %w", m.Name, err)
		}

		res.digests = append(res.digests, keyDigest{Name: m.Name, Digest: fntypes.FromHash("sha256", h).String(), Set: true})
	}

	sort.Slice(res.digests, func(i, j int) bool {
		return strings.Compare(res.digests[i].Name, res.digests[j].Name) < 0
	})

	if unsetCount == 0 {
		var err error
		res.Digest, err = digestWithInputs(typ.PkgPath(), typ.String(), res.serial, res.digests)
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}