// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package atomic

import (
	"io"
	"os"
	"path/filepath"
)

func WriteFile(dest string, r io.Reader) (int64, error) {
	tf, err := os.CreateTemp(filepath.Dir(dest), filepath.Base(dest)+".*")
	if err != nil {
		return 0, err
	}

	size, err := io.Copy(tf, r)
	if err != nil {
		tf.Close()
		os.Remove(tf.Name())
		return 0, err
	}

	if err := tf.Close(); err != nil {
		os.Remove(tf.Name())
		return 0, err
	}

	if err := os.Rename(tf.Name(), dest); err != nil {
		os.Remove(tf.Name())
		return 0, err
	}

	return size, nil
}
