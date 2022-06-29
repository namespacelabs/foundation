// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runs

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

func MakeUrl(imagePath string) (string, error) {
	content, err := ioutil.ReadFile(imagePath)
	if err != nil {
		return "", fnerrors.BadInputError("%s: failed to read: %w", imagePath, err)
	}

	clean := bytes.TrimSpace(content)

	return fmt.Sprintf("https://results.prod.namespacelabs.nscloud.dev/push/%s", base64.RawURLEncoding.EncodeToString(clean)), nil
}
