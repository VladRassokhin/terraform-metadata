package main

//noinspection GoUnresolvedReference
import (
	"github.com/hashicorp/hcl/v2/ext/customdecode"
	"github.com/hashicorp/terraform/internal/backend"
	binit "github.com/hashicorp/terraform/internal/backend/init"
	"github.com/hashicorp/terraform/internal/configs/configschema"
	"github.com/hashicorp/terraform/internal/legacy/helper/schema"
	"github.com/zclconf/go-cty/cty"

	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"unsafe"
)

// ExportSchema should be called to export the structure
// of the provisioner.
func Export(name string) *BackendSchema {
	backend := getBackend(name)
	if backend == nil {
		panic(fmt.Errorf("backend '%s' not found", name))
	}
	result := new(BackendSchema)
	result.SchemaVersion = "2"
	result.SDKType = "__SDK__"

	result.Name = "__NAME__"
	result.Type = "backend"
	result.Version = "__REVISION__"
	result.Schema = *backend

	return result
}

func getBackend(name string) *SchemaInfo {
	binit.Init(nil)
	fn := binit.Backend(name)
	if fn == nil {
		return nil
	}
	back := fn()

	if b := getOldStyleBackend(back); b != nil {
		im := schemaMap(b.Schema)
		info := im.Export()
		for k, v := range info {
			if v.Required {
				// It seems all 'required' parameters of backends could be
				// set up as `-backend-config="key=value"` during `terraform init`
				v.Required = false
				v.Optional = true
				info[k] = v
			}
		}
		return &info
	} else {
		s := back.ConfigSchema()
		info := exportBlock(s)
		for k, v := range info {
			if v.Required {
				// It seems all 'required' parameters of backends could be
				// set up as `-backend-config="key=value"` during `terraform init`
				v.Required = false
				v.Optional = true
				info[k] = v
			}
		}
		return &info
	}
}

func getOldStyleBackend(b backend.Backend) *schema.Backend {
	expected := reflect.TypeOf(&schema.Backend{})
	v := reflect.Indirect(reflect.ValueOf(b))
	tp := v.Type()
	for i := 0; i < tp.NumField(); i++ {
		field := tp.Field(i)
		if field.Type == expected {
			value := v.Field(i)
			return value.Interface().(*schema.Backend)
		}
	}
	return nil
}

func ExportResource(r *schema.Resource) SchemaInfo {
	return schemaMap(r.Schema).Export()
}

// schemaMap is a wrapper that adds nice functions on top of schemas.
type schemaMap map[string]*schema.Schema

// Export exports the format of this schema.
func (m schemaMap) Export() SchemaInfo {
	result := make(SchemaInfo)
	for k, v := range m {
		item := export(v)
		result[k] = item
	}
	return result
}

var envDefaultFunc = schema.EnvDefaultFunc("", nil)
var multiEnvDefaultFunc = schema.MultiEnvDefaultFunc([]string{}, nil)

func export(v *schema.Schema) SchemaDefinition {
	item := SchemaDefinition{}

	item.Type = shortenType(fmt.Sprintf("%s", v.Type))
	item.Optional = v.Optional
	item.Required = v.Required
	item.Description = v.Description
	item.InputDefault = v.InputDefault
	item.Computed = v.Computed
	item.MaxItems = v.MaxItems
	item.MinItems = v.MinItems
	item.PromoteSingle = v.PromoteSingle
	item.ComputedWhen = v.ComputedWhen
	item.ConflictsWith = v.ConflictsWith
	item.Deprecated = v.Deprecated
	item.Removed = v.Removed
	item.IsBlock = false

	if defValue := v.Default; defValue != nil {
		item.Default = exportValue(defValue, fmt.Sprintf("%T", defValue))
	}
	if defFunc := v.DefaultFunc; defFunc != nil {
		if reflect.ValueOf(defFunc).Pointer() == reflect.ValueOf(envDefaultFunc).Pointer() {
			item.DefaultFunc = getEnvDefaultFuncDescription(defFunc)
		} else if reflect.ValueOf(defFunc).Pointer() == reflect.ValueOf(multiEnvDefaultFunc).Pointer() {
			item.DefaultFunc = getMultiEnvDefaultFuncDescription(defFunc)
		} else {
			item.DefaultFunc = "UNKNOWN"
		}
	}

	// Logic from schemaMap.CoreConfigSchema:
	if v.Elem == nil {
		return item
	}
	if v.Type == schema.TypeMap {
		// For TypeMap in particular, it isn't valid for Elem to be a
		// *Resource (since that would be ambiguous in flatmap) and
		// so Elem is treated as a TypeString schema if so. This matches
		// how the field readers treat this situation, for compatibility
		// with configurations targeting Terraform 0.11 and earlier.
		if _, isResource := v.Elem.(*schema.Resource); isResource {
			elem := &schema.Schema{
				Type: schema.TypeString,
			}
			item.Elem = exportValue(elem, fmt.Sprintf("%T", elem))
			return item
		}
	}
	item.Elem = exportValue(v.Elem, fmt.Sprintf("%T", v.Elem))

	switch v.ConfigMode {
	case schema.SchemaConfigModeAttr:
		item.ConfigExplicitMode = "Attr"
		return item
	case schema.SchemaConfigModeBlock:
		item.ConfigExplicitMode = "Block"
		item.IsBlock = true
		return item
	default: // SchemaConfigModeAuto, or any other invalid value
		switch v.Elem.(type) {
		case *schema.Schema, schema.ValueType:
			item.ConfigImplicitMode = "Attr"
			item.IsBlock = false
		case *schema.Resource:
			if v.Computed && !v.Optional {
				// Computed-only schemas are always handled as attributes,
				// because they never appear in configuration.
				item.ConfigImplicitMode = "ComputedBlock"
				item.IsBlock = true
				return item
			} else {
				item.ConfigImplicitMode = "Block"
				item.IsBlock = true
			}
		default:
			// Should never happen for a valid schema
			panic(fmt.Errorf("invalid Schema.Elem %#v; need *Schema, ValueType or *Resource", v.Elem))
		}
	}

	return item
}

func shortenType(value string) string {
	if len(value) > 4 && value[0:4] == "Type" {
		return value[4:]
	}
	if len(value) > 4 && value[0:4] == "cty." {
		return value[4:]
	}
	return value
}

func exportValue(value interface{}, t string) *SchemaElement {
	s2, ok := value.(*schema.Schema)
	if ok {
		return &SchemaElement{Type: "SchemaElements", ElementsType: shortenType(fmt.Sprintf("%s", s2.Type))}
	}
	r2, ok := value.(*schema.Resource)
	if ok {
		return &SchemaElement{Type: "SchemaInfo", Info: ExportResource(r2)}
	}
	vt, ok := value.(schema.ValueType)
	if ok {
		return &SchemaElement{Value: shortenType(fmt.Sprintf("%v", vt))}
	}
	// Unknown case
	return &SchemaElement{Type: t, Value: fmt.Sprintf("%v", value)}
}

//region helpers for new cty.Type schemas
func exportAttribute(v *configschema.Attribute) SchemaDefinition {
	item := SchemaDefinition{}

	//noinspection GoTypesCompatibility
	item.Type = exportType(v.Type)
	item.Optional = v.Optional
	item.Required = v.Required
	item.Description = v.Description
	item.Computed = v.Computed
	item.IsBlock = false
	item.ConfigImplicitMode = "Attr"
	return item
}

func exportType(t cty.Type) string {
	if t.IsPrimitiveType() {
		return shortenType(t.GoString())
	}
	if t.IsListType() {
		et := t.ListElementType()
		if et != nil {
			return "List(" + exportType(*et) + ")"
		}
	}
	if t.IsSetType() {
		et := t.SetElementType()
		if et != nil {
			return "Set(" + exportType(*et) + ")"
		}
	}
	if t.IsMapType() {
		et := t.MapElementType()
		if et != nil {
			return "Map(" + exportType(*et) + ")"
		}
	}
	if t == cty.DynamicPseudoType {
		return "Any"
	}
	//noinspection GoUnresolvedReference
	if t == customdecode.ExpressionClosureType {
		return "Expression"
	}
	if t == cty.NilType {
		return "NilType"
	}
	// TODO: Implement other cases

	panic(fmt.Errorf("invalid type %#v", t))
}

func exportBlock(block *configschema.Block) SchemaInfo {
	result := make(SchemaInfo)
	for name, attr := range block.Attributes {
		result[name] = exportAttribute(attr)
	}
	for name, blk := range block.BlockTypes {
		result[name] = exportNestedBlock(blk)
	}
	return result
}

func exportNestedBlock(v *configschema.NestedBlock) SchemaDefinition {
	item := SchemaDefinition{}

	item.MinItems = v.MinItems
	item.MaxItems = v.MaxItems
	switch v.Nesting {
	case configschema.NestingSingle:
		item.Type = "NestingSingle"
	case configschema.NestingGroup:
		item.Type = "NestingGroup"
	case configschema.NestingList:
		item.Type = "NestingList"
	case configschema.NestingSet:
		item.Type = "NestingSet"
	case configschema.NestingMap:
		item.Type = "NestingMap"
	}

	item.Elem = &SchemaElement{Type: "SchemaInfo", Info: exportBlock(&v.Block)}

	item.IsBlock = true
	item.ConfigImplicitMode = "Block"
	return item
}

//endregion

func Generate(name string, outputPath string) {
	outputFilePath := filepath.Join(outputPath, name+`.json`)

	if err := DoGenerate(name, outputFilePath); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(255)
	}
}

func DoGenerate(name string, outputFilePath string) error {
	provisionerJson, err := json.MarshalIndent(Export(name), "", "  ")

	if err != nil {
		return err
	}

	file, err := os.Create(outputFilePath)
	if err != nil {
		return err
	}

	defer file.Close()

	_, err = file.Write(provisionerJson)
	if err != nil {
		return err
	}

	return file.Sync()
}

type SchemaElement struct {
	// One of "schema.ValueType" or "SchemaElements" or "SchemaInfo"
	Type string `json:",omitempty"`
	// Set for simple types (from ValueType)
	Value string `json:",omitempty"`
	// Set if Type == "SchemaElements"
	ElementsType string `json:",omitempty"`
	// Set if Type == "SchemaInfo"
	Info SchemaInfo `json:",omitempty"`
}

type SchemaDefinition struct {
	Type          string `json:",omitempty"`
	Optional      bool   `json:",omitempty"`
	Required      bool   `json:",omitempty"`
	Description   string `json:",omitempty"`
	InputDefault  string `json:",omitempty"`
	Computed      bool   `json:",omitempty"`
	MaxItems      int    `json:",omitempty"`
	MinItems      int    `json:",omitempty"`
	PromoteSingle bool   `json:",omitempty"`
	DefaultFunc   string `json:",omitempty"`

	ComputedWhen  []string `json:",omitempty"`
	ConflictsWith []string `json:",omitempty"`

	Deprecated string `json:",omitempty"`
	Removed    string `json:",omitempty"`

	IsBlock bool `json:",omitempty"`

	ConfigExplicitMode string `json:",omitempty"`
	ConfigImplicitMode string `json:",omitempty"`

	Default *SchemaElement `json:",omitempty"`
	Elem    *SchemaElement `json:",omitempty"`
}

type SchemaInfo map[string]SchemaDefinition

// BackendSchema
type BackendSchema struct {
	SchemaVersion string `json:".schema_version"`
	SDKType       string `json:".sdk_type"`

	Name    string     `json:"name"`
	Type    string     `json:"type"`
	Version string     `json:"version"`
	Schema  SchemaInfo `json:"schema"`
}

func main() {
	Generate("__NAME__", "__OUT__")
}

func getEnvDefaultFuncDescription(df schema.SchemaDefaultFunc) string {
	return fmt.Sprintf("ENV(%s)", getEnvDefaultFuncArgs(df))
}
func getMultiEnvDefaultFuncDescription(df schema.SchemaDefaultFunc) string {
	return fmt.Sprintf("MENV(%s)", strings.Join(getMultiEnvDefaultFuncArgs(df), ","))
}

func getEnvDefaultFuncArgs(df schema.SchemaDefaultFunc) string {
	loc := (uintptr)(unsafe.Pointer(&df))
	context_ptr := getPointerFromLocation(loc)
	// (context_ptr) <- closure function
	// (context_ptr+(uintptr(8))) <- str address
	// (context_ptr+(uintptr(16))) <- str length
	str_addr := context_ptr + (uintptr(8))
	return getString(str_addr)
}

func getMultiEnvDefaultFuncArgs(df schema.SchemaDefaultFunc) []string {
	loc := (uintptr)(unsafe.Pointer(&df))
	context_ptr := getPointerFromLocation(loc)
	// (context_ptr) <- closure function
	// (context_ptr+(uintptr(8))) <- []str address
	// (context_ptr+(uintptr(16))) <- []str length
	// (context_ptr+(uintptr(24))) <- []str cap
	str_addr := context_ptr + (uintptr(8))
	return getSlice(str_addr)
}

func getPointerFromLocation(location uintptr) uintptr {
	return *(*uintptr)(unsafe.Pointer(location))
}

func getString(addr uintptr) string {
	SH := (*reflect.StringHeader)(unsafe.Pointer(addr))

	var res string
	pString := (*reflect.StringHeader)(unsafe.Pointer(&res))

	pString.Data = SH.Data
	pString.Len = SH.Len
	return res
}

func getSlice(addr uintptr) []string {
	SH := (*reflect.SliceHeader)(unsafe.Pointer(addr))

	var res = make([]string, 3)
	pString := (*reflect.SliceHeader)(unsafe.Pointer(&res))

	pString.Data = SH.Data
	pString.Len = SH.Len
	pString.Cap = SH.Len
	return res
}
