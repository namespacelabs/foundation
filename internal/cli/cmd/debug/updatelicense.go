// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package debug

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
)

func newUpdateLicenseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "update-license",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			var paths []string

			fsys := os.DirFS(".")

			if err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}

				if d.Name() == "node_modules" && d.IsDir() {
					return fs.SkipDir
				}

				if d.IsDir() {
					return nil
				}

				if slices.Contains([]string{".go", ".js", ".ts", ".jsx", ".tsx", ".proto", ".hcl", ".yaml", ".yml", ".css"}, filepath.Ext(path)) {
					paths = append(paths, path)
				}
				return nil
			}); err != nil {
				return err
			}

			var headers []struct {
				target string
				prefix []byte
			}

			for _, lic := range []string{apacheLicense, earlyAccessLicense} {
				for _, p := range []func(string) []byte{cppComment, cComment, shellComment} {
					headers = append(headers, struct {
						target string
						prefix []byte
					}{lic, p(lic)})
				}
			}

			const target = earlyAccessLicense

		file:
			for _, path := range paths {
				contents, err := fs.ReadFile(fsys, path)
				if err != nil {
					return err
				}

				for _, h := range headers {
					if bytes.HasPrefix(contents, h.prefix) {
						if h.target == target {
							continue file
						}

						updated := bytes.TrimSpace(bytes.TrimPrefix(contents, h.prefix))
						if err := os.WriteFile(path, updated, 0644); err != nil {
							return err
						}
						break
					}
				}

				p := extensions[filepath.Ext(path)]
				if p != nil {
					gen := p(target)
					if err := os.WriteFile(path, append(gen, contents...), 0644); err != nil {
						return err
					}
				}
			}

			return nil
		}),
	}

	return cmd
}

var extensions = map[string]func(string) []byte{
	".go":    cppComment,
	".js":    cppComment,
	".ts":    cppComment,
	".tsx":   cppComment,
	".jsx":   cppComment,
	".proto": cppComment,
}

const apacheLicense = `Copyright 2022 Namespace Labs Inc

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.`

const earlyAccessLicense = `Copyright 2022 Namespace Labs Inc; All rights reserved.
Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
available at http://github.com/namespacelabs/foundation`

func cppComment(license string) []byte {
	lines := strings.Split(strings.TrimSpace(license), "\n")
	for k, line := range lines {
		lines[k] = "// " + line
	}
	return []byte(strings.Join(lines, "\n") + "\n\n")
}

func shellComment(license string) []byte {
	lines := strings.Split(strings.TrimSpace(license), "\n")
	for k, line := range lines {
		lines[k] = "# " + line
	}
	return []byte(strings.Join(lines, "\n") + "\n\n")
}

func cComment(license string) []byte {
	lines := strings.Split(strings.TrimSpace(license), "\n")
	for k, line := range lines {
		lines[k] = " * " + line
	}
	allLines := append([]string{"/**"}, lines...)
	allLines = append(allLines, " */")
	return []byte(strings.Join(allLines, "\n") + "\n\n")
}
