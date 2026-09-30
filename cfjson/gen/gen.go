// Package gen generates JSON encoders and decoders for Go structs, built on
// top of the cfjson runtime package.
package gen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// DefaultRuntime is the import path of the runtime package generated code
// uses, unless Config.Runtime says otherwise.
const DefaultRuntime = "github.com/centrifugal/protocol/cfjson"

// Config describes what to generate.
type Config struct {
	// Files are the Go source files with the structs, all of one package. They
	// are only read: generated code goes to a file of its own, so a package
	// can get JSON code without a change to its sources.
	Files []string
	// Types limits generation to the listed structs. All structs of Files are
	// used if it is empty.
	Types []string
	// RawTypes are names of []byte based types which hold an already encoded
	// JSON value, like json.RawMessage (which is always treated as one). A
	// type of another package is named with its package: "pkg.Type".
	RawTypes []string
	// AppendMethod and DecodeMethod are the names of generated methods,
	// AppendJSON and DecodeJSON by default. Use unexported names to keep the
	// methods out of the package API.
	AppendMethod string
	DecodeMethod string
	// Runtime is the import path of the cfjson runtime package.
	Runtime string
	// FoldKeys makes decoders match a key which is not the name of a field
	// again without regard to the case of ASCII letters, the way encoding/json
	// does: {"Channel": ...} then sets the field channel. It is for input
	// written by hand or by clients which have always been allowed to do
	// that. A key still may not repeat, in whatever case it is written.
	FoldKeys bool
	// NoNilElements makes decoders read a null which is an element of a
	// slice of pointers to structs, or a value of a map of them, as an empty
	// struct. encoding/json leaves nil there, and nil is what code walking
	// over a decoded message tends not to expect: Protobuf, the other
	// encoding of the same messages, has no way to say it.
	NoNilElements bool
}

type kind int

const (
	kString kind = iota
	kBool
	kInt
	kUint
	kFloat
	kRaw
	kStruct
	kPtr
	kSlice
	kMap
	// kCustom is a type which has both MarshalJSON and UnmarshalJSON, and so
	// is encoded and decoded by calling them, whatever it is made of.
	kCustom
)

// typ is a field type as far as the generator cares.
type typ struct {
	kind kind
	// src is the type as written in the source, e.g. "map[string]*ClientInfo".
	src string
	// named is set for basic kinds declared as a named type, such values
	// need a conversion.
	named bool
	// float32 tells float32 from float64.
	float32 bool
	// elem is the element type of kPtr, kSlice and kMap.
	elem *typ
	// keyNamed is set if the key of a kMap is a named string type, keySrc is
	// then its name.
	keyNamed bool
	keySrc   string
	// pkg and name tell which struct a kStruct is.
	pkg  *pkgInfo
	name string
	// isByte is set for uint8 and the types based on it.
	isByte bool
	// marshaler and unmarshaler are set for a named type which has the
	// MarshalJSON or the UnmarshalJSON method: that direction is then left
	// to the method.
	marshaler, unmarshaler bool
	// empty is the kind deciding when a kCustom value is empty for omitempty.
	empty kind
}

// guard is a pointer to an embedded struct on the way to a field: it may be
// nil, in which case the field is not there to encode, and has to be
// allocated before the field is decoded into.
type guard struct {
	// expr is the pointer, like "m.Options".
	expr string
	// src is the type it points to.
	src string
}

type field struct {
	// path is how the field is reached from the struct: its name, or for a
	// field of an embedded struct the names leading to it, "Options.Info".
	path      string
	jsonName  string
	omitEmpty bool
	typ       *typ
	guards    []guard
	// depth is how many structs the field is embedded through, tagged says
	// that its name comes from a tag: encoding/json picks one of the fields
	// with the same name by these.
	depth  int
	tagged bool
}

type structInfo struct {
	// name is the type as generated code refers to it: "Claims" for a struct
	// of the package code is generated for, "jwt.Claims" for one of another.
	name string
	pkg  *pkgInfo
	// typeName is the name without the package.
	typeName string
	fields   []field
	// recursive is set if a value of the struct may have a value of the same
	// type inside.
	recursive bool
}

// fileImports are the imports of a source file: what a qualified name in a
// type declared in that file refers to.
type fileImports struct {
	// named maps the names given to imports to their paths.
	named map[string]string
	// unnamed are the paths imported under the name of the package, which
	// takes asking the go command to find out.
	unnamed  []string
	resolved bool
}

// decl is a type declaration.
type decl struct {
	expr    ast.Expr
	imports *fileImports
}

// pkgInfo is what the generator knows about a package: the one code is
// generated for, or one a type of which is used in a field.
type pkgInfo struct {
	// path is the import path, empty for the package code is generated for.
	path  string
	name  string
	local bool
	decls map[string]*decl
	// methods has the names of the JSON methods types of the package have.
	methods map[string]map[string]bool
}

type scope struct {
	pkg     *pkgInfo
	imports *fileImports
}

type generator struct {
	cfg   Config
	local *pkgInfo
	// dir is where the go command is run to find other packages: the
	// directory of the input files, so that it sees their module.
	dir  string
	pkgs map[string]*pkgInfo
	// importNames are the names packages are imported under in the generated
	// file, by path.
	importNames map[string]string
	// structs are the structs code is generated for, by name as in
	// structInfo, and queue is the order they were found in.
	structs map[string]*structInfo
	queue   []*structInfo
	raw     map[string]bool
	// embedding is set while the type of an embedded field is resolved: its
	// fields become fields of the struct it is embedded in, so no code of
	// its own is needed for it.
	embedding bool
	buf       bytes.Buffer
	nvar      int
}

var basicKinds = map[string]kind{
	"string": kString, "bool": kBool,
	"int": kInt, "int8": kInt, "int16": kInt, "int32": kInt, "int64": kInt,
	"uint": kUint, "uint8": kUint, "uint16": kUint, "uint32": kUint, "uint64": kUint, "uintptr": kUint,
	"float32": kFloat, "float64": kFloat,
}

// Generate returns the source of a Go file with methods for the structs cfg
// selects.
func Generate(cfg Config) ([]byte, error) {
	if cfg.AppendMethod == "" {
		cfg.AppendMethod = "AppendJSON"
	}
	if cfg.DecodeMethod == "" {
		cfg.DecodeMethod = "DecodeJSON"
	}
	if cfg.Runtime == "" {
		cfg.Runtime = DefaultRuntime
	}
	if len(cfg.Files) == 0 {
		return nil, fmt.Errorf("no input files")
	}
	g := &generator{
		cfg:         cfg,
		dir:         filepath.Dir(cfg.Files[0]),
		pkgs:        map[string]*pkgInfo{},
		importNames: map[string]string{},
		structs:     map[string]*structInfo{},
		raw:         map[string]bool{},
	}
	for _, name := range cfg.RawTypes {
		g.raw[name] = true
	}

	local, order, err := parsePackage("", cfg.Files)
	if err != nil {
		return nil, err
	}
	local.local = true
	g.local = local
	if methodsErr := siblingMethods(local, cfg.Files); methodsErr != nil {
		return nil, methodsErr
	}
	if len(cfg.Types) > 0 {
		for _, name := range cfg.Types {
			d := local.decls[name]
			if d == nil {
				return nil, fmt.Errorf("type %s is not a struct declared in the input files", name)
			}
			if _, ok := d.expr.(*ast.StructType); !ok {
				return nil, fmt.Errorf("type %s is not a struct declared in the input files", name)
			}
		}
		order = cfg.Types
	}
	for _, name := range order {
		if g.raw[name] {
			continue
		}
		if _, ok := local.decls[name].expr.(*ast.StructType); !ok {
			continue
		}
		if addErr := g.addStruct(local, name); addErr != nil {
			return nil, addErr
		}
	}
	// Resolving the fields of a struct may add structs of other packages to
	// the queue, so it grows while it is walked.
	for n := 0; n < len(g.queue); n++ {
		if fillErr := g.fillStruct(g.queue[n]); fillErr != nil {
			return nil, fillErr
		}
	}
	markRecursive(g.queue)

	for _, si := range g.queue {
		g.encoder(si)
		g.decoder(si)
	}
	body := g.buf.String()

	var out bytes.Buffer
	out.WriteString("// Code generated by cfjson. DO NOT EDIT.\n\n")
	fmt.Fprintf(&out, "package %s\n\nimport (\n", local.name)
	paths := make([]string, 0, len(g.importNames))
	for path := range g.importNames {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		// A package may have been looked at without anything of it being
		// named in the code.
		name := g.importNames[path]
		if regexp.MustCompile(`(^|[^A-Za-z0-9_.])` + regexp.QuoteMeta(name) + `\.`).MatchString(body) {
			fmt.Fprintf(&out, "\t%s %q\n", name, path)
		}
	}
	fmt.Fprintf(&out, "\n\tcfjson %q\n)\n", cfg.Runtime)
	out.WriteString(body)
	src, err := format.Source(out.Bytes())
	if err != nil {
		return out.Bytes(), fmt.Errorf("generated code does not compile, this is a bug in cfjson: %w", err)
	}
	return src, nil
}

var generatedFile = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`)

// siblingMethods adds the methods declared in the files of the package which
// were not given to the generator: a type is encoded by its MarshalJSON
// wherever in the package the method is written. Generated files are not
// looked at, methods there are not what somebody wrote for a type (and the
// file this generator wrote before is one of them).
func siblingMethods(p *pkgInfo, files []string) error {
	given, dirs := map[string]bool{}, map[string]bool{}
	for _, name := range files {
		abs, err := filepath.Abs(name)
		if err != nil {
			return err
		}
		given[abs] = true
		dirs[filepath.Dir(abs)] = true
	}
	fset := token.NewFileSet()
	for dir := range dirs {
		names, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			return err
		}
		for _, name := range names {
			if given[name] || strings.HasSuffix(name, "_test.go") {
				continue
			}
			src, err := os.ReadFile(name) //nolint:gosec // G304: the files next to the input files.
			if err != nil {
				return err
			}
			if generatedFile.Match(src) {
				continue
			}
			f, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
			if err != nil || f.Name.Name != p.name {
				// Not a file of this package, or not one which builds.
				continue
			}
			for _, d := range f.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok {
					p.addMethod(fd)
				}
			}
		}
	}
	return nil
}

// addMethod records a method of a type, whatever the receiver is: the
// generator looks for the JSON ones.
func (p *pkgInfo) addMethod(d *ast.FuncDecl) {
	if d.Recv == nil || len(d.Recv.List) != 1 {
		return
	}
	recv := d.Recv.List[0].Type
	if star, ok := recv.(*ast.StarExpr); ok {
		recv = star.X
	}
	if id, ok := recv.(*ast.Ident); ok {
		if p.methods[id.Name] == nil {
			p.methods[id.Name] = map[string]bool{}
		}
		p.methods[id.Name][d.Name.Name] = true
	}
}

// parsePackage reads the type declarations of the given files of one
// package. It returns the names of the types in the order they are declared.
func parsePackage(path string, files []string) (*pkgInfo, []string, error) {
	p := &pkgInfo{path: path, decls: map[string]*decl{}, methods: map[string]map[string]bool{}}
	var order []string
	fset := token.NewFileSet()
	for _, name := range files {
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, nil, err
		}
		if p.name != "" && p.name != f.Name.Name {
			return nil, nil, fmt.Errorf("files belong to different packages: %s and %s", p.name, f.Name.Name)
		}
		p.name = f.Name.Name
		imports := &fileImports{named: map[string]string{}}
		for _, spec := range f.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return nil, nil, err
			}
			switch {
			case spec.Name == nil:
				imports.unnamed = append(imports.unnamed, importPath)
			case spec.Name.Name != "_" && spec.Name.Name != ".":
				imports.named[spec.Name.Name] = importPath
			}
		}
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.GenDecl:
				if d.Tok != token.TYPE {
					continue
				}
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok || ts.TypeParams != nil {
						continue
					}
					p.decls[ts.Name.Name] = &decl{expr: ts.Type, imports: imports}
					order = append(order, ts.Name.Name)
				}
			case *ast.FuncDecl:
				p.addMethod(d)
			}
		}
	}
	if p.name == "" {
		return nil, nil, fmt.Errorf("no input files")
	}
	return p, order, nil
}

// load returns the package with the given import path, reading its sources
// the first time.
func (g *generator) load(path string) (*pkgInfo, error) {
	if p, ok := g.pkgs[path]; ok {
		return p, nil
	}
	// The go command knows where a package is, whether it comes from the
	// standard library, the module or one of its dependencies.
	cmd := exec.Command("go", "list", "-f", `{{.Dir}}{{range .GoFiles}}|{{.}}{{end}}`, path) //nolint:gosec // G204: the path comes from the imports of the input files.
	cmd.Dir = g.dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list %s: %w", path, err)
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "|")
	if len(parts) < 2 {
		return nil, fmt.Errorf("package %s has no Go files", path)
	}
	files := make([]string, 0, len(parts)-1)
	for _, name := range parts[1:] {
		files = append(files, filepath.Join(parts[0], name))
	}
	p, _, err := parsePackage(path, files)
	if err != nil {
		return nil, err
	}
	g.pkgs[path] = p
	return p, nil
}

// importPath returns the path of the package a file refers to by name.
func (g *generator) importPath(imports *fileImports, name string) (string, error) {
	if path, ok := imports.named[name]; ok {
		return path, nil
	}
	if !imports.resolved && len(imports.unnamed) > 0 {
		// The name of a package is not always the last element of its path
		// (think of a /v5), so ask.
		args := append([]string{"list", "-f", "{{.ImportPath}}|{{.Name}}"}, imports.unnamed...)
		cmd := exec.Command("go", args...) //nolint:gosec // G204: the paths come from the imports of the input files.
		cmd.Dir = g.dir
		out, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("go list %s: %w", strings.Join(imports.unnamed, " "), err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if path, pkgName, ok := strings.Cut(line, "|"); ok {
				if _, taken := imports.named[pkgName]; !taken {
					imports.named[pkgName] = path
				}
			}
		}
		imports.resolved = true
	}
	if path, ok := imports.named[name]; ok {
		return path, nil
	}
	return "", fmt.Errorf("unknown package %s", name)
}

// qual returns the name generated code refers to a type by.
func (g *generator) qual(p *pkgInfo, name string) string {
	if p.local {
		return name
	}
	importName, ok := g.importNames[p.path]
	if !ok {
		importName = p.name
		for n := 2; g.importNameTaken(importName); n++ {
			importName = p.name + strconv.Itoa(n)
		}
		g.importNames[p.path] = importName
	}
	return importName + "." + name
}

func (g *generator) importNameTaken(name string) bool {
	if name == "cfjson" {
		return true
	}
	for _, taken := range g.importNames {
		if taken == name {
			return true
		}
	}
	return false
}

// addStruct puts a struct on the list of structs code is generated for.
func (g *generator) addStruct(p *pkgInfo, name string) error {
	key := g.qual(p, name)
	if g.structs[key] != nil {
		return nil
	}
	if !p.local && !ast.IsExported(name) {
		return fmt.Errorf("type %s of package %s is not exported", name, p.path)
	}
	si := &structInfo{name: key, pkg: p, typeName: name}
	g.structs[key] = si
	g.queue = append(g.queue, si)
	return nil
}

func (g *generator) fillStruct(si *structInfo) error {
	d := si.pkg.decls[si.typeName]
	st, _ := d.expr.(*ast.StructType)
	fields, err := g.fields(si.name, st, scope{pkg: si.pkg, imports: d.imports}, "", nil, 0)
	if err != nil {
		return err
	}
	if len(fields) == 0 && !si.pkg.local {
		// Its state is private: {} would be written for whatever it holds.
		return fmt.Errorf("type %s has no exported fields and no JSON methods", si.name)
	}
	si.fields = fields
	return nil
}

// fields returns the fields of a struct the way encoding/json sees them: its
// own, and those of the structs embedded in it, which count as fields of the
// struct itself. prefix and guards say how the struct is reached when it is
// an embedded one.
func (g *generator) fields(name string, st *ast.StructType, sc scope, prefix string, guards []guard, depth int) ([]field, error) {
	if depth > 10 {
		return nil, fmt.Errorf("%s: structs are embedded too deep", name)
	}
	// all has the fields in the order encoding/json writes them: the order
	// of declaration, with the fields of an embedded struct where the struct
	// is embedded.
	var all []field
	seen := map[string]bool{}
	for _, f := range st.Fields.List {
		var tag string
		if f.Tag != nil {
			unquoted, err := strconv.Unquote(f.Tag.Value)
			if err != nil {
				return nil, fmt.Errorf("%s: bad struct tag %s", name, f.Tag.Value)
			}
			tag = reflect.StructTag(unquoted).Get("json")
		}
		if tag == "-" {
			continue
		}
		jsonName, opts, _ := strings.Cut(tag, ",")
		omitEmpty := false
		for opts != "" {
			var opt string
			opt, opts, _ = strings.Cut(opts, ",")
			switch opt {
			case "omitempty":
				omitEmpty = true
			default:
				return nil, fmt.Errorf("%s: unsupported json tag option %q", name, opt)
			}
		}

		names := make([]string, 0, len(f.Names))
		for _, ident := range f.Names {
			names = append(names, ident.Name)
		}
		if len(f.Names) == 0 {
			embedded, isPtr := f.Type, false
			if star, ok := embedded.(*ast.StarExpr); ok {
				embedded, isPtr = star.X, true
			}
			var embeddedName string
			switch e := embedded.(type) {
			case *ast.Ident:
				embeddedName = e.Name
			case *ast.SelectorExpr:
				embeddedName = e.Sel.Name
			default:
				return nil, fmt.Errorf("%s: unsupported embedded field", name)
			}
			if jsonName == "" {
				// An embedded struct without a name of its own: its fields
				// are fields of this struct.
				// What is embedded in a struct of another package under a
				// name which is not exported cannot be reached from here.
				if !sc.pkg.local && !ast.IsExported(embeddedName) {
					continue
				}
				g.embedding = true
				t, err := g.resolve(embedded, sc, 0)
				g.embedding = false
				if err != nil {
					return nil, fmt.Errorf("%s.%s: %w", name, embeddedName, err)
				}
				if t.kind != kStruct || t.marshaler || t.unmarshaler {
					return nil, fmt.Errorf("%s.%s: only structs without JSON methods of their own can be embedded", name, embeddedName)
				}
				inner := t.pkg.decls[t.name]
				innerStruct, _ := inner.expr.(*ast.StructType)
				innerGuards := guards
				if isPtr {
					innerGuards = append(append([]guard{}, guards...), guard{expr: "m." + prefix + embeddedName, src: t.src})
				}
				fields, err := g.fields(name, innerStruct, scope{pkg: t.pkg, imports: inner.imports}, prefix+embeddedName+".", innerGuards, depth+1)
				if err != nil {
					return nil, err
				}
				all = append(all, fields...)
				continue
			}
			// With a name it is a field like any other.
			names = append(names, embeddedName)
		}
		for _, goName := range names {
			if !ast.IsExported(goName) {
				continue
			}
			t, err := g.resolve(f.Type, sc, 0)
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", name, prefix+goName, err)
			}
			fd := field{path: prefix + goName, jsonName: jsonName, omitEmpty: omitEmpty, typ: t, guards: guards, depth: depth, tagged: jsonName != ""}
			if fd.jsonName == "" {
				fd.jsonName = goName
			}
			if !plainKey(fd.jsonName) {
				return nil, fmt.Errorf("%s.%s: JSON name %q has characters which need escaping", name, prefix+goName, fd.jsonName)
			}
			if seen[fd.jsonName] {
				return nil, fmt.Errorf("%s: duplicate JSON field name %q", name, fd.jsonName)
			}
			seen[fd.jsonName] = true
			all = append(all, fd)
		}
	}
	if depth > 0 {
		// Which of the fields with the same name is the field is decided
		// for the whole struct at once.
		return all, nil
	}
	// The rule of encoding/json: of the fields with the same name the one
	// which is embedded the least deep is the field. When there are several
	// that deep, it is the one which got its name from a tag, if there is
	// exactly one, and otherwise the struct has no field of this name.
	type choice struct{ depth, count, tagged, index int }
	best := map[string]*choice{}
	for n, fd := range all {
		c := best[fd.jsonName]
		if c == nil || fd.depth < c.depth {
			c = &choice{depth: fd.depth, index: -1}
			best[fd.jsonName] = c
		}
		if fd.depth > c.depth {
			continue
		}
		c.count++
		if fd.tagged {
			c.tagged++
			c.index = n
		} else if c.tagged == 0 {
			c.index = n
		}
	}
	fields := all[:0]
	for n, fd := range all {
		if c := best[fd.jsonName]; c.index == n && (c.count == 1 || c.tagged == 1) {
			fields = append(fields, fd)
		}
	}
	return fields, nil
}

// plainKey reports whether a JSON name can be written as it is: generated
// code writes names as literals, without escaping.
func plainKey(name string) bool {
	for _, r := range name {
		if r < 0x20 || r == '"' || r == '\\' || r == '<' || r == '>' || r == '&' || r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// emptyKind says what makes a value of a declared type empty, judging by
// what the type is made of.
func (g *generator) emptyKind(expr ast.Expr, sc scope, depth int) kind {
	if depth > 10 {
		return kStruct
	}
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return g.emptyKind(e.X, sc, depth+1)
	case *ast.Ident:
		if d, ok := sc.pkg.decls[e.Name]; ok {
			return g.emptyKind(d.expr, scope{pkg: sc.pkg, imports: d.imports}, depth+1)
		}
		if k, ok := basicKinds[e.Name]; ok {
			return k
		}
	case *ast.StarExpr:
		return kPtr
	case *ast.ArrayType:
		if e.Len == nil {
			return kSlice
		}
	case *ast.MapType:
		return kMap
	}
	// A struct, and whatever else: never empty.
	return kStruct
}

// named resolves a type declared in a package.
func (g *generator) named(p *pkgInfo, name string, depth int) (*typ, error) {
	d := p.decls[name]
	src := g.qual(p, name)
	if p.path == "encoding/json" && name == "RawMessage" {
		return &typ{kind: kRaw, src: src}, nil
	}
	if (p.local && g.raw[name]) || (!p.local && g.raw[p.name+"."+name]) {
		return &typ{kind: kRaw, src: src}, nil
	}
	if !p.local && !ast.IsExported(name) {
		return nil, fmt.Errorf("type %s of package %s is not exported", name, p.path)
	}
	marshaler, unmarshaler := p.methods[name]["MarshalJSON"], p.methods[name]["UnmarshalJSON"]
	sc := scope{pkg: p, imports: d.imports}
	if marshaler && unmarshaler {
		return &typ{kind: kCustom, src: src, marshaler: true, unmarshaler: true, empty: g.emptyKind(d.expr, sc, 0)}, nil
	}
	// encoding/json writes such a type as the string it makes of itself.
	// Reading its fields instead would be another encoding, and for most of
	// these types there are no fields to read.
	if p.methods[name]["MarshalText"] || p.methods[name]["UnmarshalText"] {
		return nil, fmt.Errorf("type %s encodes itself as text (MarshalText), which is not supported", src)
	}
	if _, ok := d.expr.(*ast.StructType); ok {
		t := &typ{kind: kStruct, src: src, pkg: p, name: name, marshaler: marshaler, unmarshaler: unmarshaler, empty: kStruct}
		// A struct of another package gets functions generated next to the
		// methods of the structs of this one. A struct of this package
		// which was not asked for gets its methods generated as well,
		// unless the input files have them written by hand.
		written := p.local && p.methods[name][g.cfg.AppendMethod] && p.methods[name][g.cfg.DecodeMethod]
		if !written && !g.embedding {
			if err := g.addStruct(p, name); err != nil {
				return nil, err
			}
		}
		return t, nil
	}
	under, err := g.resolve(d.expr, sc, depth+1)
	if err != nil {
		return nil, err
	}
	t := *under
	t.src = src
	if t.kind <= kFloat {
		t.named = true
	}
	t.marshaler, t.unmarshaler = marshaler, unmarshaler
	t.empty = g.emptyKind(d.expr, sc, 0)
	return &t, nil
}

func (g *generator) resolve(expr ast.Expr, sc scope, depth int) (*typ, error) {
	if depth > 20 {
		return nil, fmt.Errorf("type is declared in terms of itself")
	}
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return g.resolve(e.X, sc, depth)
	case *ast.Ident:
		// A raw type may be declared in a file which was not given.
		if sc.pkg.local && g.raw[e.Name] {
			return &typ{kind: kRaw, src: e.Name}, nil
		}
		if _, ok := sc.pkg.decls[e.Name]; ok {
			return g.named(sc.pkg, e.Name, depth)
		}
		if k, ok := basicKinds[e.Name]; ok {
			return &typ{kind: k, src: e.Name, float32: e.Name == "float32", isByte: e.Name == "uint8"}, nil
		}
		if e.Name == "byte" {
			return &typ{kind: kUint, src: e.Name, isByte: true}, nil
		}
		return nil, fmt.Errorf("unsupported type %s", e.Name)
	case *ast.SelectorExpr:
		x, ok := e.X.(*ast.Ident)
		if !ok {
			return nil, fmt.Errorf("unsupported type %s", types.ExprString(expr))
		}
		path, err := g.importPath(sc.imports, x.Name)
		if err != nil {
			return nil, err
		}
		p, err := g.load(path)
		if err != nil {
			return nil, err
		}
		if _, ok := p.decls[e.Sel.Name]; !ok {
			return nil, fmt.Errorf("unsupported type %s.%s", x.Name, e.Sel.Name)
		}
		return g.named(p, e.Sel.Name, depth)
	case *ast.StarExpr:
		elem, err := g.resolve(e.X, sc, depth)
		if err != nil {
			return nil, err
		}
		return &typ{kind: kPtr, src: "*" + elem.src, elem: elem}, nil
	case *ast.ArrayType:
		if e.Len != nil {
			return nil, fmt.Errorf("unsupported array type %s", types.ExprString(expr))
		}
		if id, ok := e.Elt.(*ast.Ident); ok && (id.Name == "byte" || id.Name == "uint8") {
			return nil, fmt.Errorf("unsupported type []byte, declare a named type and pass it in -raw")
		}
		elem, err := g.resolve(e.Elt, sc, depth)
		if err != nil {
			return nil, err
		}
		if elem.isByte && !elem.marshaler {
			// encoding/json writes a slice of bytes as base64, whatever
			// the name of the byte type is.
			return nil, fmt.Errorf("unsupported type %s, a slice of bytes", types.ExprString(expr))
		}
		return &typ{kind: kSlice, src: "[]" + elem.src, elem: elem}, nil
	case *ast.MapType:
		key, err := g.resolve(e.Key, sc, depth)
		if err != nil {
			return nil, err
		}
		if key.kind != kString || key.marshaler || key.unmarshaler {
			return nil, fmt.Errorf("unsupported map key type %s, only strings are supported", key.src)
		}
		elem, err := g.resolve(e.Value, sc, depth)
		if err != nil {
			return nil, err
		}
		return &typ{kind: kMap, src: "map[" + key.src + "]" + elem.src, elem: elem, keyNamed: key.named, keySrc: key.src}, nil
	}
	return nil, fmt.Errorf("unsupported type %s", types.ExprString(expr))
}

// structName returns the name of the struct values of type t are made of, if
// there is one.
func structName(t *typ) string {
	for t.elem != nil {
		t = t.elem
	}
	if t.kind == kStruct {
		return t.src
	}
	return ""
}

// markRecursive finds the structs which can be reached from themselves.
func markRecursive(structs []*structInfo) {
	edges := map[string][]string{}
	for _, si := range structs {
		for _, f := range si.fields {
			if name := structName(f.typ); name != "" {
				edges[si.name] = append(edges[si.name], name)
			}
		}
	}
	for _, si := range structs {
		seen := map[string]bool{}
		var visit func(name string) bool
		visit = func(name string) bool {
			for _, next := range edges[name] {
				if next == si.name {
					return true
				}
				if !seen[next] {
					seen[next] = true
					if visit(next) {
						return true
					}
				}
			}
			return false
		}
		si.recursive = visit(si.name)
	}
}

func (g *generator) p(line string, args ...any) {
	if len(args) == 0 {
		g.buf.WriteString(line)
	} else {
		fmt.Fprintf(&g.buf, line, args...)
	}
	g.buf.WriteByte('\n')
}

// v returns a variable name which is unique within the generated file.
func (g *generator) v(prefix string) string {
	g.nvar++
	return prefix + strconv.Itoa(g.nvar)
}

// funcName returns the name of the function generated for a struct of
// another package, which cannot be given methods.
func (g *generator) funcName(verb string, t *typ) string {
	importName := g.importNames[t.pkg.path]
	return verb + strings.ToUpper(importName[:1]) + importName[1:] + t.name
}

// receiver returns what a method is called on for the value x: x itself,
// or p when x is written as "(*p)".
func receiver(x string) string {
	if strings.HasPrefix(x, "(*") && strings.HasSuffix(x, ")") {
		return x[2 : len(x)-1]
	}
	return x
}

// appendStruct returns the expression appending the struct x.
func (g *generator) appendStruct(t *typ, x string) string {
	if t.pkg.local {
		return fmt.Sprintf("%s.%s(b)", receiver(x), g.cfg.AppendMethod)
	}
	return fmt.Sprintf("%s(b, %s)", g.funcName("appendJSON", t), addr(x))
}

// decodeStruct returns the expression decoding into the struct x.
func (g *generator) decodeStruct(t *typ, x string) string {
	if t.pkg.local {
		return fmt.Sprintf("%s.%s(b, i, f)", receiver(x), g.cfg.DecodeMethod)
	}
	return fmt.Sprintf("%s(%s, b, i, f)", g.funcName("decodeJSON", t), addr(x))
}

// keyLiteral returns a Go string literal with the given prefix followed by
// the object key for a field and a colon.
func keyLiteral(prefix, name string) string {
	return strconv.Quote(prefix + `"` + name + `":`)
}

// emptyCond returns the condition under which a field with omitempty is
// written, or nothing if it always is.
func emptyCond(t *typ, x string) string {
	k := t.kind
	if k == kCustom || t.marshaler {
		k = t.empty
	}
	switch k {
	case kString:
		return x + ` != ""`
	case kBool:
		return x
	case kInt, kUint, kFloat:
		return x + " != 0"
	case kPtr:
		return x + " != nil"
	case kRaw, kSlice, kMap:
		return "len(" + x + ") != 0"
	}
	return ""
}

func (g *generator) encoder(si *structInfo) {
	g.nvar = 0
	g.p("")
	if si.pkg.local {
		g.p("// %s appends the JSON encoding of m to b.", g.cfg.AppendMethod)
		g.p("func (m *%s) %s(b []byte) []byte {", si.name, g.cfg.AppendMethod)
	} else {
		fn := g.funcName("appendJSON", &typ{pkg: si.pkg, name: si.typeName})
		g.p("// %s appends the JSON encoding of m to b.", fn)
		g.p("func %s(b []byte, m *%s) []byte {", fn, si.name)
	}
	if len(si.fields) == 0 {
		g.p("return append(b, '{', '}')")
		g.p("}")
		return
	}
	// When the first field is always there the output starts with `{"name":`
	// and every other field with `,"name":`. Otherwise every field starts
	// with a comma, and the first comma written is turned into the opening
	// brace at the end – this avoids tracking "is this the first field" at
	// run time.
	patch := si.fields[0].omitEmpty || len(si.fields[0].guards) > 0
	if patch {
		g.p("s := len(b)")
	}
	for n, f := range si.fields {
		prefix := ","
		if n == 0 && !patch {
			prefix = "{"
		}
		x := "m." + f.path
		conds := make([]string, 0, len(f.guards)+1)
		// A field of an embedded struct which is not there is not written.
		for _, gd := range f.guards {
			conds = append(conds, gd.expr+" != nil")
		}
		nonEmpty := false
		if f.omitEmpty {
			if cond := emptyCond(f.typ, x); cond != "" {
				conds = append(conds, cond)
				nonEmpty = true
			}
		}
		if len(conds) > 0 {
			g.p("if %s {", strings.Join(conds, " && "))
		}
		if nonEmpty && f.typ.kind == kBool && !f.typ.marshaler {
			// A bool which is written only when it is not empty is true:
			// the value goes into the literal with the key.
			g.p("b = append(b, %q...)", prefix+`"`+f.jsonName+`":true`)
		} else {
			g.p("b = append(b, %s...)", keyLiteral(prefix, f.jsonName))
			g.encodeValue(f.typ, x, nonEmpty)
		}
		if len(conds) > 0 {
			g.p("}")
		}
	}
	if patch {
		g.p("if len(b) == s {")
		g.p("return append(b, '{', '}')")
		g.p("}")
		g.p("b[s] = '{'")
	}
	g.p("return append(b, '}')")
	g.p("}")
}

// encodeValue emits code appending the value of expression x of type t.
// nonEmpty is set if x is known to be a non-nil pointer or a non-empty slice
// or map.
func (g *generator) encodeValue(t *typ, x string, nonEmpty bool) {
	// A type with a MarshalJSON method says itself what its JSON is.
	if t.kind == kCustom || t.marshaler {
		g.p("b = cfjson.AppendMarshaler(b, %s)", addr(x))
		return
	}
	conv := func(to string) string {
		if t.named || to != t.src {
			return to + "(" + x + ")"
		}
		return x
	}
	switch t.kind {
	case kString:
		g.p("b = cfjson.AppendString(b, %s)", conv("string"))
	case kBool:
		g.p("b = cfjson.AppendBool(b, %s)", conv("bool"))
	case kInt:
		g.p("b = cfjson.AppendInt(b, %s)", conv("int64"))
	case kUint:
		g.p("b = cfjson.AppendUint(b, %s)", conv("uint64"))
	case kFloat:
		if t.float32 {
			g.p("b = cfjson.AppendFloat32(b, %s)", conv("float32"))
		} else {
			g.p("b = cfjson.AppendFloat64(b, %s)", conv("float64"))
		}
	case kRaw:
		g.p("b = cfjson.AppendRaw(b, %s)", x)
	case kStruct:
		g.p("b = %s", g.appendStruct(t, x))
	case kPtr:
		if !nonEmpty {
			g.p("if %s == nil {", x)
			g.p(`b = append(b, "null"...)`)
			g.p("} else {")
		}
		g.encodeValue(t.elem, "(*"+x+")", false)
		if !nonEmpty {
			g.p("}")
		}
	case kSlice:
		if !nonEmpty {
			g.p("if %s == nil {", x)
			g.p(`b = append(b, "null"...)`)
			g.p("} else {")
		}
		n, e := g.v("n"), g.v("e")
		g.p("b = append(b, '[')")
		g.p("for %s, %s := range %s {", n, e, x)
		g.p("if %s != 0 {", n)
		g.p("b = append(b, ',')")
		g.p("}")
		g.encodeValue(t.elem, e, false)
		g.p("}")
		g.p("b = append(b, ']')")
		if !nonEmpty {
			g.p("}")
		}
	case kMap:
		if !nonEmpty {
			g.p("if %s == nil {", x)
			g.p(`b = append(b, "null"...)`)
			g.p("} else {")
		}
		// Same trick as for struct fields: every entry starts with a comma,
		// and the first one is turned into the opening brace.
		s, k, e := g.v("s"), g.v("k"), g.v("e")
		g.p("%s := len(b)", s)
		g.p("for %s, %s := range %s {", k, e, x)
		g.p("b = append(b, ',')")
		if t.keyNamed {
			g.p("b = cfjson.AppendString(b, string(%s))", k)
		} else {
			g.p("b = cfjson.AppendString(b, %s)", k)
		}
		g.p("b = append(b, ':')")
		g.encodeValue(t.elem, e, false)
		g.p("}")
		if nonEmpty {
			g.p("b[%s] = '{'", s)
		} else {
			g.p("if len(b) == %s {", s)
			g.p("b = append(b, '{')")
			g.p("} else {")
			g.p("b[%s] = '{'", s)
			g.p("}")
		}
		g.p("b = append(b, '}')")
		if !nonEmpty {
			g.p("}")
		}
	}
}

func (g *generator) decoder(si *structInfo) {
	g.nvar = 0
	g.p("")
	if si.pkg.local {
		g.p("// %s decodes the JSON value at b[i] into m and returns the index after the", g.cfg.DecodeMethod)
		g.p("// value, or a negative number on error, see cfjson.Error.")
		g.p("func (m *%s) %s(b []byte, i int, f cfjson.Flags) int {", si.name, g.cfg.DecodeMethod)
	} else {
		fn := g.funcName("decodeJSON", &typ{pkg: si.pkg, name: si.typeName})
		g.p("// %s decodes the JSON value at b[i] into m and returns the index after", fn)
		g.p("// the value, or a negative number on error, see cfjson.Error.")
		g.p("func %s(m *%s, b []byte, i int, f cfjson.Flags) int {", fn, si.name)
	}
	g.p("if i >= len(b) || b[i] != '{' {")
	g.p("return cfjson.Null(b, i)")
	g.p("}")
	if si.recursive {
		// The type can nest in itself, so the input decides how deep the
		// recursion goes. Bound it.
		g.p("if f += cfjson.DepthStep; f >= cfjson.DepthLimit {")
		g.p("return ^i")
		g.p("}")
	}
	g.p("i = cfjson.SkipSpace(b, i+1)")
	g.p("if i < len(b) && b[i] == '}' {")
	g.p("return i + 1")
	g.p("}")
	// A bit per field, set when the field has been decoded: a key may only
	// come once. Two parsers which pick different ones of a repeated key read
	// different messages out of the same bytes, so such input is refused.
	words := (len(si.fields) + 63) / 64
	for w := 0; w < words; w++ {
		g.p("var seen%d uint64", w)
	}
	if len(si.fields) == 0 {
		g.p("for {")
		g.p("_, i = cfjson.Key(b, i, f)")
		g.p("if i < 0 {")
		g.p("return i")
		g.p("}")
		g.p("i = cfjson.Skip(b, i, f)")
	} else {
		// Which field a key names is found in two steps: the number of the
		// field first, what to do with it after. Encoders write fields in
		// the order they are declared in, so the key which is most likely to
		// come is the one of the field after the last one: it is compared
		// with as it is written, quotes and colon included, which takes a
		// couple of word compares and no look at the key itself. Whatever
		// else comes is read as a key and looked up.
		g.p("next := 0")
		g.p("for {")
		g.p("id := -1")
		// A switch with one case is an if, for those who read the code and
		// for the linters.
		single := len(si.fields) == 1
		if !single {
			g.p("switch next {")
		}
		for n, f := range si.fields {
			lit := `"` + f.jsonName + `":`
			if single {
				g.p("if next == 0 && len(b)-i >= %d && string(b[i:i+%d]) == %q {", len(lit), len(lit), lit)
			} else {
				g.p("case %d:", n)
				g.p("if len(b)-i >= %d && string(b[i:i+%d]) == %q {", len(lit), len(lit), lit)
			}
			g.p("id = %d", n)
			g.p("i = cfjson.SkipSpace(b, i+%d)", len(lit))
			g.p("}")
		}
		if !single {
			g.p("}")
		}
		g.p("if id < 0 {")
		g.p("var key []byte")
		g.p("key, i = cfjson.Key(b, i, f)")
		g.p("if i < 0 {")
		g.p("return i")
		g.p("}")
		// Keys are matched exactly. The compiler does not allocate for the
		// conversion in a switch.
		if g.cfg.FoldKeys {
			g.p("folded := false")
			g.p("match:")
		}
		if single && !g.cfg.FoldKeys {
			g.p("if string(key) == %q {", si.fields[0].jsonName)
			g.p("id = 0")
			g.p("}")
		} else {
			g.p("switch string(key) {")
			for n, f := range si.fields {
				g.p("case %q:", f.jsonName)
				g.p("id = %d", n)
			}
			if g.cfg.FoldKeys {
				g.p("default:")
				g.foldKey(si)
			}
			g.p("}")
		}
		g.p("}")
		g.p("switch id {")
		for n, f := range si.fields {
			g.p("case %d:", n)
			g.p("if seen%d&%#x != 0 {", n/64, uint64(1)<<(n%64))
			g.p("return ^i")
			g.p("}")
			g.p("seen%d |= %#x", n/64, uint64(1)<<(n%64))
			// The embedded structs a field belongs to come into being with
			// the first of their fields.
			for _, gd := range f.guards {
				g.p("if %s == nil {", gd.expr)
				g.p("%s = new(%s)", gd.expr, gd.src)
				g.p("}")
			}
			g.decodeValue(f.typ, "m."+f.path, false)
			g.p("next = %d", n+1)
		}
		g.p("default:")
		g.p("i = cfjson.Skip(b, i, f)")
		g.p("}")
	}
	g.p("if i < 0 {")
	g.p("return i")
	g.p("}")
	g.p("i = cfjson.SkipSpace(b, i)")
	g.p("if i >= len(b) {")
	g.p("return ^i")
	g.p("}")
	g.p("switch b[i] {")
	g.p("case ',':")
	g.p("i = cfjson.SkipSpace(b, i+1)")
	g.p("case '}':")
	g.p("return i + 1")
	g.p("default:")
	g.p("return ^i")
	g.p("}")
	g.p("}")
	g.p("}")
}

// foldKey emits what a decoder does with a key which is not the name of a
// field when keys are to be matched without regard to letter case: it tries
// once more with the key in lower case. This is off the path of keys written
// the way fields are named, which costs them nothing.
func (g *generator) foldKey(si *structInfo) {
	lower := func(name string) string {
		return strings.Map(func(r rune) rune {
			if 'A' <= r && r <= 'Z' {
				r += 'a' - 'A'
			}
			return r
		}, name)
	}
	allLower := true
	for _, f := range si.fields {
		allLower = allLower && lower(f.jsonName) == f.jsonName
	}
	g.p("if !folded {")
	g.p("folded = true")
	if allLower {
		// Field names are all in lower case, so the folded key can be looked
		// up with the same switch.
		g.p("if k, ok := cfjson.FoldKey(key); ok {")
		g.p("key = k")
		g.p("goto match")
		g.p("}")
	} else {
		g.p("k, _ := cfjson.FoldKey(key)")
		g.p("switch string(k) {")
		seen := map[string]bool{}
		for _, f := range si.fields {
			k := lower(f.jsonName)
			if seen[k] {
				continue
			}
			seen[k] = true
			g.p("case %q:", k)
			g.p("key = []byte(%q)", f.jsonName)
			g.p("goto match")
		}
		g.p("}")
	}
	g.p("}")
}

// addr returns an expression for the address of x, which is either an
// addressable expression or "(*p)" for a pointer p.
func addr(x string) string {
	if strings.HasPrefix(x, "(*") && strings.HasSuffix(x, ")") {
		return x[2 : len(x)-1]
	}
	return "&" + x
}

// decodeValue emits code decoding the value at b[i] into the addressable
// expression x of type t, leaving the index after the value (or an error) in i.
// fresh is set if x is a variable which has just been declared, and so is
// known to hold the zero value.
func (g *generator) decodeValue(t *typ, x string, fresh bool) {
	// A type with an UnmarshalJSON method is handed the value as it is.
	if t.kind == kCustom || t.unmarshaler {
		g.p("i = cfjson.DecodeUnmarshaler(b, i, f, %s)", addr(x))
		return
	}
	switch t.kind {
	case kString:
		if t.named {
			g.p("i = cfjson.String(b, i, f, (*string)(%s))", addr(x))
		} else {
			g.p("i = cfjson.String(b, i, f, %s)", addr(x))
		}
	case kBool:
		g.p("i = cfjson.Bool(b, i, %s)", addr(x))
	case kInt:
		g.p("i = cfjson.Int(b, i, %s)", addr(x))
	case kUint:
		g.p("i = cfjson.Uint(b, i, %s)", addr(x))
	case kFloat:
		g.p("i = cfjson.Float(b, i, %s)", addr(x))
	case kRaw:
		g.p("i = cfjson.Raw(b, i, f, (*[]byte)(%s))", addr(x))
	case kStruct:
		g.p("i = %s", g.decodeStruct(t, x))
	case kPtr:
		g.p("if cfjson.IsNull(b, i) {")
		switch {
		case !fresh:
			g.p("%s = nil", x)
		case g.cfg.NoNilElements && t.elem.kind == kStruct:
			g.p("%s = new(%s)", x, t.elem.src)
		}
		g.p("i += 4")
		g.p("} else {")
		if fresh {
			g.p("%s = new(%s)", x, t.elem.src)
		} else {
			g.p("if %s == nil {", x)
			g.p("%s = new(%s)", x, t.elem.src)
			g.p("}")
		}
		g.decodeValue(t.elem, "(*"+x+")", fresh)
		g.p("}")
	case kSlice:
		e := g.v("e")
		g.p("if cfjson.IsNull(b, i) {")
		if !fresh {
			g.p("%s = nil", x)
		}
		g.p("i += 4")
		g.p("} else {")
		g.p("if i >= len(b) || b[i] != '[' {")
		g.p("return ^i")
		g.p("}")
		g.p("i = cfjson.SkipSpace(b, i+1)")
		g.p("if i < len(b) && b[i] == ']' {")
		g.p("i++")
		if fresh {
			g.p("%s = %s{}", x, t.src)
		} else {
			g.p("if %s == nil {", x)
			g.p("%s = %s{}", x, t.src)
			g.p("} else {")
			g.p("%s = %s[:0]", x, x)
			g.p("}")
		}
		g.p("} else {")
		// Start with room for a few elements rather than growing from one,
		// which saves allocations. 10 is what segmentio starts with, so the
		// number of allocations for a slice stays what it was.
		const initial = 10
		if fresh {
			g.p("%s = make(%s, 0, %d)", x, t.src, initial)
		} else {
			g.p("if cap(%s) == 0 {", x)
			g.p("%s = make(%s, 0, %d)", x, t.src, initial)
			g.p("} else {")
			g.p("%s = %s[:0]", x, x)
			g.p("}")
		}
		g.p("for {")
		g.p("var %s %s", e, t.elem.src)
		g.decodeValue(t.elem, e, true)
		g.p("if i < 0 {")
		g.p("return i")
		g.p("}")
		g.p("%s = append(%s, %s)", x, x, e)
		g.p("i = cfjson.SkipSpace(b, i)")
		g.p("if i >= len(b) {")
		g.p("return ^i")
		g.p("}")
		g.p("if b[i] == ',' {")
		g.p("i = cfjson.SkipSpace(b, i+1)")
		g.p("continue")
		g.p("}")
		g.p("if b[i] != ']' {")
		g.p("return ^i")
		g.p("}")
		g.p("i++")
		g.p("break")
		g.p("}")
		g.p("}")
		g.p("}")
	case kMap:
		k, e := g.v("k"), g.v("e")
		g.p("if cfjson.IsNull(b, i) {")
		if !fresh {
			g.p("%s = nil", x)
		}
		g.p("i += 4")
		g.p("} else {")
		g.p("if i >= len(b) || b[i] != '{' {")
		g.p("return ^i")
		g.p("}")
		// A map is replaced rather than added to: its keys may not repeat
		// either, and what was in it before is not a part of this object.
		g.p("%s = make(%s)", x, t.src)
		g.p("i = cfjson.SkipSpace(b, i+1)")
		g.p("if i < len(b) && b[i] == '}' {")
		g.p("i++")
		g.p("} else {")
		count := g.v("n")
		g.p("%s := 0", count)
		g.p("for {")
		g.p("var %s string", k)
		g.p("i = cfjson.MapKey(b, i, f, &%s)", k)
		g.p("if i < 0 {")
		g.p("return i")
		g.p("}")
		g.p("var %s %s", e, t.elem.src)
		g.decodeValue(t.elem, e, true)
		g.p("if i < 0 {")
		g.p("return i")
		g.p("}")
		key := k
		if t.keyNamed {
			key = t.keySrc + "(" + k + ")"
		}
		// The map was made above, so it has one entry per key stored so
		// far unless a key came twice. Counting is one lookup less per entry
		// than asking the map before storing.
		g.p("%s[%s] = %s", x, key, e)
		g.p("%s++", count)
		g.p("if len(%s) != %s {", x, count)
		g.p("return ^i")
		g.p("}")
		g.p("i = cfjson.SkipSpace(b, i)")
		g.p("if i >= len(b) {")
		g.p("return ^i")
		g.p("}")
		g.p("if b[i] == ',' {")
		g.p("i = cfjson.SkipSpace(b, i+1)")
		g.p("continue")
		g.p("}")
		g.p("if b[i] != '}' {")
		g.p("return ^i")
		g.p("}")
		g.p("i++")
		g.p("break")
		g.p("}")
		g.p("}")
		g.p("}")
	}
}
