// Command cfprotobuf generates Protobuf marshalers and unmarshalers for the
// structs protoc-gen-go produces.
//
//	cfprotobuf -out client.pb_cfprotobuf.go client.pb.go
//
// See the cfprotobuf package documentation for what the generated methods do.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/centrifugal/protocol/cfprotobuf/gen"
)

func main() {
	var cfg gen.Config
	var out, typeNames string
	flag.StringVar(&out, "out", "", "output file, defaults to the first input file with the _cfprotobuf.go suffix")
	flag.StringVar(&typeNames, "types", "", "comma-separated names of structs to generate code for, all messages by default")
	flag.StringVar(&cfg.Suffix, "suffix", "CF", "what the names of generated methods end with: MarshalCF, UnmarshalCF and so on")
	flag.StringVar(&cfg.Runtime, "runtime", gen.DefaultRuntime, "import path of the cfprotobuf runtime package")
	flag.BoolVar(&cfg.DropUnknown, "drop-unknown", false, "skip fields a message does not have when decoding, instead of keeping them with the message")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: cfprotobuf [flags] file.go...")
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
	if out == "" {
		out = strings.TrimSuffix(cfg.Files[0], ".go") + "_cfprotobuf.go"
	}
	src, err := gen.Generate(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cfprotobuf:", err)
		os.Exit(1)
	}
	// The output is source code, to be read by whoever reads the rest of it.
	if err := os.WriteFile(out, src, 0o644); err != nil { //nolint:gosec // G306: see above.
		fmt.Fprintln(os.Stderr, "cfprotobuf:", err)
		os.Exit(1)
	}
}
