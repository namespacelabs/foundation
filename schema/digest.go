// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package schema

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"strings"

	"google.golang.org/protobuf/proto"
)

type Digest struct {
	// Algorithm holds the algorithm used to compute the hash.
	Algorithm string

	// Hex holds the hex portion of the content hash.
	Hex string
}

func (v Digest) IsSet() bool { return v.Hex != "" }

func (v Digest) String() string {
	if !v.IsSet() {
		return ""
	}
	return fmt.Sprintf("%s:%s", v.Algorithm, v.Hex)
}

func (v Digest) Equals(rhs Digest) bool {
	return v.Algorithm == rhs.Algorithm && v.Hex == rhs.Hex
}

// Implement `compute.Digestible`.
func (v Digest) ComputeDigest(context.Context) (Digest, error) { return v, nil }

func ParseDigest(str string) (Digest, error) {
	var d Digest
	return d, parseDigest(&d, str)
}

func parseDigest(d *Digest, str string) error {
	parts := strings.SplitN(str, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("%s: invalid digest", str)
	}

	// XXX validate format.
	d.Algorithm = parts[0]
	d.Hex = parts[1]

	return nil
}

func DigestOf(vals ...interface{}) (Digest, error) {
	return bits("sha256", sha256.New(), vals...)
}

func bits(algo string, h hash.Hash, vals ...interface{}) (Digest, error) {
	for _, v := range vals {
		if err := serializeBytes(h, v); err != nil {
			return Digest{}, err
		}
	}
	return FromHash(algo, h), nil
}

func FromHash(algo string, h hash.Hash) Digest {
	return Digest{Algorithm: algo, Hex: hex.EncodeToString(h.Sum(nil))}
}

func serializeBytes(w io.Writer, v interface{}) error {
	var b []byte
	var err error
	switch x := v.(type) {
	case proto.Message:
		b, err = (proto.MarshalOptions{Deterministic: true}).Marshal(x)
	case []byte:
		b = x
	default:
		b, err = json.Marshal(v)
	}

	if err != nil {
		return err
	}

	_, err = w.Write(b)
	return err
}
