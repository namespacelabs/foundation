// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package debug

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnfs"
)

var checkExtensions = []string{".go", ".js", ".ts", ".jsx", ".tsx", ".proto", ".hcl", ".yaml", ".yml", ".css"}
var ignoreSuffixes = []string{
	".pb.go",
	".fn.go",
	".pb.gw.go",
	".fn.ts",
	".fn.js",
	"_pb.ts",
	"stacktrace/stacktrace.go",
	"stacktrace/serializer/serializer.go",
	// Compiled Namespace plugin for Yarn.
	"plugin-fn.js"}

func newUpdateLicenseCmd() *cobra.Command {
	var check bool

	cmd := &cobra.Command{
		Use: "update-license",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			fsys := os.DirFS(".")

			paths := args
			if len(paths) == 0 {
				if err := fnfs.WalkDir(fsys, ".", func(path string, d fs.DirEntry) error {
					if d.Name() == "." {
						return nil
					}

					if len(d.Name()) > 0 && d.Name()[0] == '.' {
						if d.IsDir() {
							return fs.SkipDir
						}
						return nil
					}

					if d.IsDir() {
						// "dist" contains compiled Typescript code, for example, std/nodejs/runtime.
						if d.Name() == "node_modules" || d.Name() == "dist" {
							return fs.SkipDir
						}

						return nil
					}

					for _, ignore := range ignoreSuffixes {
						if strings.HasSuffix(path, ignore) {
							return nil
						}
					}

					if slices.Contains(checkExtensions, filepath.Ext(path)) {
						paths = append(paths, path)
					}
					return nil
				}); err != nil {
					return err
				}
			}

			var headers []struct {
				target string
				prefix []byte
			}

			for _, lic := range []string{apacheLicense, earlyAccessLicense, shortApacheLicense} {
				for _, prependComment := range []func(string) []byte{cppComment, cComment, shellComment} {
					headers = append(headers, struct {
						target string
						prefix []byte
					}{lic, prependComment(lic)})
				}
			}

			const target = shortApacheLicense

			var wouldWrite []string

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

						contents = bytes.TrimPrefix(contents, h.prefix)
						break
					}
				}

				for len(contents) > 0 && contents[0] == '\n' {
					contents = contents[1:]
				}

				makeHeader := extensions[filepath.Ext(path)]
				if makeHeader != nil {
					if check {
						wouldWrite = append(wouldWrite, path)
					} else {
						gen := makeHeader(target)
						gen = append(gen, '\n')
						if err := os.WriteFile(path, append(gen, contents...), 0644); err != nil {
							return err
						}
					}
				}
			}

			if len(wouldWrite) > 0 {
				return fmt.Errorf("the following files need their license header updated:\n%s", strings.Join(wouldWrite, "\n"))
			}

			return nil
		}),
	}

	cmd.Flags().BoolVar(&check, "check", check, "If set to true, check that all files have the appropriate header.")

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

const shortApacheLicense = `Copyright 2022 Namespace Labs Inc; All rights reserved.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.`

func cppComment(license string) []byte {
	lines := strings.Split(strings.TrimSpace(license), "\n")
	for k, line := range lines {
		lines[k] = prepend("//", line)
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func shellComment(license string) []byte {
	lines := strings.Split(strings.TrimSpace(license), "\n")
	for k, line := range lines {
		lines[k] = prepend("#", line)
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func cComment(license string) []byte {
	lines := strings.Split(strings.TrimSpace(license), "\n")
	for k, line := range lines {
		lines[k] = prepend(" *", line)
	}
	allLines := append([]string{"/**"}, lines...)
	allLines = append(allLines, " */")
	return []byte(strings.Join(allLines, "\n") + "\n")
}

func prepend(prefix, line string) string {
	if line == "" {
		return prefix
	}
	return prefix + " " + line
}
