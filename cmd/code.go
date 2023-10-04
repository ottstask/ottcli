package cmd

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"io/ioutil"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type arrayFlags []string

func (i *arrayFlags) String() string {
	return "my string representation"
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

// read openapi and generate code for models and operations
// example gogen http://localhost:8080/api/my-server/v1/api.json -operation ListExample
func main() {
	operations := arrayFlags{}
	var module, lang string
	flag.Var(&operations, "operation", "Operation list (default all)")
	flag.StringVar(&module, "module", "client", "Module name")
	flag.StringVar(&lang, "lang", "go", "Module name")
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		fmt.Println("Must set one openapi url")
		return
	}
	openapiURL := args[0]

	uri, err := url.Parse(openapiURL)
	if err != nil {
		fmt.Println("Bad format of url", openapiURL, err)
		return
	}

	opMap := make(map[string]bool)
	for _, v := range operations {
		opMap[v] = true
	}

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromURI(uri)
	if err != nil {
		fmt.Println("LoadFromURI error", err)
		return
	}
	// sort
	sortedPath := make([]string, 0)
	opIds := make(map[string]bool)
	for p, pp := range doc.Paths {
		if pp.Post == nil {
			continue
		}
		sortedPath = append(sortedPath, p)
		opIds[pp.Post.OperationID] = true
	}

	for op := range opMap {
		if !opIds[op] {
			fmt.Println("missing operation", op)
			return
		}
	}

	sortedSchemas := make([]string, 0)
	for k := range doc.Components.Schemas {
		sortedSchemas = append(sortedSchemas, k)
	}
	sort.Strings(sortedPath)
	sort.Strings(sortedSchemas)

	switch lang {
	case "go":
		goGen(openapiURL, module, sortedPath, sortedSchemas, doc, opMap)
	case "js":
		jsGen(openapiURL, module, sortedPath, sortedSchemas, doc, opMap)
	case "py":
		pyGen(openapiURL, module, sortedPath, sortedSchemas, doc, opMap)
	default:
		fmt.Println("Unsupported lang", lang)
		return
	}

}

func goGen(openapiURL, module string, sortedPath, sortedSchemas []string, doc *openapi3.T, opMap map[string]bool) {
	header := fmt.Sprintf(`// This file was generated from %s
package %s

`, openapiURL, module)

	// Gen model code
	out := bytes.NewBuffer(nil)
	fmt.Fprint(out, header)
	for _, key := range sortedSchemas {
		sm := doc.Components.Schemas[key]
		fmt.Fprintf(out, "type %s struct{\n", key)
		requiredFirstKeys, requiredKeys := getRequiredFirstKeys(sm)
		for _, pr := range requiredFirstKeys {
			va := sm.Value.Properties[pr]
			isRequired := ""
			if requiredKeys[pr] {
				isRequired = "Required. "
			}
			if va.Value.Title != "" || isRequired != "" {
				fmt.Fprintf(out, "// %s%s\n", isRequired, va.Value.Title)
			}
			fmt.Fprintf(out, "%s %s\n", toName(pr, "go"), toType(va, "go"))
		}
		fmt.Fprint(out, "}\n\n")
	}

	content, err := format.Source(out.Bytes())
	checkError(err)
	err = ioutil.WriteFile(module+".model.gen.go", content, 0644)
	checkError(err)

	// Gen client code
	out = bytes.NewBuffer(nil)
	fmt.Fprint(out, header)

	fmt.Fprintf(out, `import "context"
	var client apiClient
	type apiClient interface {
	Invoke(context.Context, string, interface{}, interface{}) error 
	}
	func SetApiClient(c apiClient){
	client = c
	}
	
	`)

	for _, path := range sortedPath {
		pp := doc.Paths[path]
		if pp.Post == nil {
			continue
		}
		op := pp.Post.OperationID
		if len(opMap) > 0 && !opMap[op] {
			continue
		}

		if pp.Post.Summary != "" {
			fmt.Fprintf(out, "// %s\n", pp.Post.Summary)
		}
		fmt.Fprintf(out, "func %s(ctx context.Context, req *%sRequest) (*%sResponse, error){\n", op, op, op)
		fmt.Fprintf(out, "rsp := &%sResponse{}\n", op)
		fmt.Fprintf(out, "err := client.Invoke(ctx, \"%s\", req, rsp)\n", path)
		fmt.Fprintf(out, "return rsp, err\n")
		fmt.Fprintf(out, "}\n\n")
	}

	content, err = format.Source(out.Bytes())
	checkError(err)
	err = ioutil.WriteFile(module+".client.gen.go", content, 0644)
	checkError(err)
}

func pyGen(openapiURL, module string, sortedPath, sortedSchemas []string, doc *openapi3.T, opMap map[string]bool) {
	header := fmt.Sprintf(`# This file was generated from %s
`, openapiURL)

	// Gen model code
	out := bytes.NewBuffer(nil)
	fmt.Fprint(out, header)
	fmt.Fprint(out, "\n")
	for _, key := range sortedSchemas {
		sm := doc.Components.Schemas[key]
		fmt.Fprintf(out, "# %s: %s\n", key, getSchemaExample(sm, "py"))
	}
	err := ioutil.WriteFile(module+"_model.py", out.Bytes(), 0644)
	checkError(err)

	// Gen client code
	out = bytes.NewBuffer(nil)
	fmt.Fprint(out, header)

	fmt.Fprintf(out, `from abc import ABCMeta, abstractmethod


class InvokeClass(metaclass=ABCMeta):
    @abstractmethod
    def invoke(self, path: str, req):
        pass


client: InvokeClass


def set_api_client(c: InvokeClass):
    global client
    client = c


`)
	for _, path := range sortedPath {
		pp := doc.Paths[path]
		if pp.Post == nil {
			continue
		}
		op := pp.Post.OperationID
		if len(opMap) > 0 && !opMap[op] {
			continue
		}

		if pp.Post.Summary != "" {
			fmt.Fprintf(out, "# %s\n", pp.Post.Summary)
		}
		fmt.Fprintf(out, "def %s(req):\n", toName(op, "py"))
		fmt.Fprintf(out, "    # req: %s\n", getSchemaExample(doc.Components.Schemas[op+"Request"], "py"))
		fmt.Fprintf(out, "    # rsp: %s\n", getSchemaExample(doc.Components.Schemas[op+"Response"], "py"))
		fmt.Fprintf(out, "    return client.invoke(\"%s\", req)\n", path)
		fmt.Fprintf(out, "\n\n")
	}
	err = ioutil.WriteFile(module+"_client.py", out.Bytes(), 0644)
	checkError(err)
}

func jsGen(openapiURL, module string, sortedPath, sortedSchemas []string, doc *openapi3.T, opMap map[string]bool) {
	header := fmt.Sprintf(`// This file was generated from %s
`, openapiURL)

	// Gen model code
	out := bytes.NewBuffer(nil)
	fmt.Fprint(out, header)
	fmt.Fprint(out, "\n")
	for _, key := range sortedSchemas {
		sm := doc.Components.Schemas[key]
		fmt.Fprintf(out, "// %s: %s\n", key, getSchemaExample(sm, "js"))
	}
	err := ioutil.WriteFile(module+"_model.js", out.Bytes(), 0644)
	checkError(err)

	// Gen client code
	out = bytes.NewBuffer(nil)
	fmt.Fprint(out, header)

	className := cases.Title(language.English).String(module) + "API"

	fmt.Fprintf(out, `import %s_client_service from '@/service/%s_client'

class %s {
`, module, module, className)

	for _, path := range sortedPath {
		pp := doc.Paths[path]
		if pp.Post == nil {
			continue
		}
		op := pp.Post.OperationID
		if len(opMap) > 0 && !opMap[op] {
			continue
		}

		if pp.Post.Summary != "" {
			fmt.Fprintf(out, "  // %s\n", pp.Post.Summary)
		}
		fmt.Fprintf(out, "  // req: %s\n", getSchemaExample(doc.Components.Schemas[op+"Request"], "js"))
		fmt.Fprintf(out, "  // rsp: %s\n", getSchemaExample(doc.Components.Schemas[op+"Response"], "js"))
		fmt.Fprintf(out, `  %s = (data) => {
    return %s_client_service({
      url: '%s',
      method: 'post',
      data: data,
    })
  }
`, strings.ToLower(op[:1])+op[1:], module, path)
	}
	footer := fmt.Sprintf(`
}

export default new %s()
`, className)

	fmt.Fprint(out, footer)
	err = ioutil.WriteFile(module+"_client.js", out.Bytes(), 0644)
	checkError(err)

}

func getSchemaExample(ref *openapi3.SchemaRef, lang string) string {
	if ref.Ref != "" {
		return `{imposible value}`
	}
	allKeys, reqiuredKeys := getRequiredFirstKeys(ref)

	ret := bytes.NewBuffer(nil)
	ret.WriteString("{")
	for i, key := range allKeys {
		tf := toType(ref.Value.Properties[key], lang)
		title := ""
		if ref.Value.Properties[key].Value != nil && ref.Value.Properties[key].Value.Title != "" {
			title = " " + ref.Value.Properties[key].Value.Title
		}
		if reqiuredKeys[key] {
			fmt.Fprintf(ret, `"%s": "type %s(required).%s"`, key, tf, title)
		} else {
			fmt.Fprintf(ret, `"%s": "type %s.%s"`, key, tf, title)
		}
		if i+1 != len(allKeys) {
			ret.WriteString(", ")
		}
	}
	ret.WriteString("}")
	return ret.String()
}

func getRequiredFirstKeys(sm *openapi3.SchemaRef) ([]string, map[string]bool) {
	sortKeys := make([]string, 0)
	for k := range sm.Value.Properties {
		sortKeys = append(sortKeys, k)
	}
	requiredKeys := make(map[string]bool)
	for _, v := range sm.Value.Required {
		requiredKeys[v] = true
	}
	sort.Strings(sortKeys)
	requiredFirstKeys := make([]string, 0)
	for _, v := range sortKeys {
		if !requiredKeys[v] {
			continue
		}
		requiredFirstKeys = append(requiredFirstKeys, v)
	}
	for _, v := range sortKeys {
		if requiredKeys[v] {
			continue
		}
		requiredFirstKeys = append(requiredFirstKeys, v)
	}
	return requiredFirstKeys, requiredKeys
}

func toType(t *openapi3.SchemaRef, lang string) string {
	if t.Ref != "" {
		name := strings.TrimPrefix(t.Ref, "#/components/schemas/")
		if lang == "go" {
			return "*" + name
		}
		return name
	}
	var mapType string
	var arrayType string

	if t.Value.Type == "object" && t.Value.AdditionalProperties.Schema != nil {
		mapType = toType(t.Value.AdditionalProperties.Schema, lang)
	}

	if t.Value.Type == "array" {
		arrayType = toType(t.Value.Items, lang)
	}

	typeMap := map[string]string{
		"go-string":  "string",
		"go-boolean": "bool",
		"go-number":  "float64",
		"go-integer": "int64",
		"go-array":   "[]" + arrayType,
		"go-object":  "map[string]" + mapType,

		"py-string":  "str",
		"py-boolean": "bool",
		"py-number":  "float",
		"py-integer": "int",
		"py-array":   "List[" + arrayType + "]",
		"py-object":  "Dict[str, " + mapType + "]",

		"js-string":  "string",
		"js-boolean": "boolean",
		"js-number":  "float",
		"js-integer": "int",
		"js-array":   "array[" + arrayType + "]",
		"js-object":  "map[string, " + mapType + "]",
	}
	if v, ok := typeMap[lang+"-"+t.Value.Type]; ok {
		return v
	}
	panic("unknown type:" + lang + "-" + t.Value.Type)

}

func toName(name, lang string) string {
	switch lang {
	case "py":
		return toSnakeCase(name)
	case "go":
		return cases.Title(language.English, cases.NoLower).String(name)
	default:
	}
	return name
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func toSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}
