// Command cfjson generates JSON encoders and decoders for the structs of a Go
// package.
//
//	cfjson -raw Raw -out client.pb_cfjson.go client.pb.go
//
// See the cfjson package documentation for what the generated methods do.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/centrifugal/protocol/cfjson/gen"
)

func main() {
	var cfg gen.Config
	var out, typeNames, raw string
	flag.StringVar(&out, "out", "", "output file, defaults to the first input file with the _cfjson.go suffix")
	flag.StringVar(&typeNames, "types", "", "comma-separated names of structs to generate code for, all structs by default")
	flag.StringVar(&raw, "raw", "", "comma-separated names of []byte based types holding raw JSON, like json.RawMessage")
	flag.StringVar(&cfg.AppendMethod, "append", "AppendJSON", "name of the generated encoding method")
	flag.StringVar(&cfg.DecodeMethod, "decode", "DecodeJSON", "name of the generated decoding method")
	flag.StringVar(&cfg.Runtime, "runtime", gen.DefaultRuntime, "import path of the cfjson runtime package")
	flag.BoolVar(&cfg.FoldKeys, "fold-keys", false, "match a key which is not the name of a field again without regard to letter case, like encoding/json")
	flag.BoolVar(&cfg.NoNilElements, "no-nil-elements", false, "decode null in a slice or a map of pointers to structs as an empty struct, not as nil")
	flag.StringVar(&cfg.ValidRawMethod, "valid-raw", "", "generate one more method with this name, which reports whether the raw JSON values in a struct are valid")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: cfjson [flags] file.go...")
		flag.PrintDefaults()
	}
	flag.Parse()
	cfg.Files = flag.Args()
	if len(cfg.Files) == 0 {
		flag.Usage()
		os.Exit(2)
	}
	if typeNames != "" {
		cfg.Types = strings.Split(typeNames, ",")
	}
	if raw != "" {
		cfg.RawTypes = strings.Split(raw, ",")
	}
	if out == "" {
		out = strings.TrimSuffix(cfg.Files[0], ".go") + "_cfjson.go"
	}
	cfg.Out = out
	src, err := gen.Generate(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cfjson:", err)
		os.Exit(1)
	}
	// The output is source code, to be read by whoever reads the rest of it.
	if err := os.WriteFile(out, src, 0o644); err != nil { //nolint:gosec // G306: see above.
		fmt.Fprintln(os.Stderr, "cfjson:", err)
		os.Exit(1)
	}
}
