package main

//noinspection GoUnresolvedReference
import (
	prvdr "__REPOSITORY__/__PKG_NAME__"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	tf "github.com/hashicorp/terraform-plugin-sdk/terraform"

	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
	"unsafe"
)

// ExportSchema should be called to export the structure
// of the provider.
func Export(p *schema.Provider) *ResourceProviderSchema {
	result := new(ResourceProviderSchema)
	result.SchemaVersion = "2"
	result.SDKType = "terraform-plugin-sdk-v1"

	result.Name = "__NAME__"
	result.Type = "provider"
	result.Version = "__REVISION__"
	result.Provider = schemaMap(p.Schema).Export()
	result.Resources = make(map[string]SchemaInfoWithTimeouts)
	result.DataSources = make(map[string]SchemaInfoWithTimeouts)

	for k, r := range p.ResourcesMap {
		result.Resources[k] = ExportResourceWithTimeouts(r)
	}
	for k, ds := range p.DataSourcesMap {
		result.DataSources[k] = ExportResourceWithTimeouts(ds)
	}

	return result
}

func ExportResourceWithTimeouts(r *schema.Resource) SchemaInfoWithTimeouts {
	var timeouts []string
	if t := r.Timeouts; t != nil {
		tp := reflect.ValueOf(*t)
		for i := 0; i < tp.NumField(); i++ {
			field := tp.Type().Field(i)
			val := tp.Field(i)
			if field.Type == reflect.PtrTo(reflect.TypeOf(time.Nanosecond)) && !val.IsNil() {
				timeouts = append(timeouts, strings.ToLower(field.Name))
			}
		}
	}
	result := make(SchemaInfoWithTimeouts)
	for nk, nv := range ExportResource(r) {
		result[nk] = nv
	}
	if len(timeouts) > 0 {
		result["__timeouts__"] = timeouts
	}
	return result
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
		item.Default = exportDefaultValue(defValue)
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
	panic(fmt.Errorf("unexpected case: %T; %s; %#v;", value, t, value))
}

func exportDefaultValue(value interface{}) *SchemaElement {
	if _, ok := value.(*schema.Schema); ok {
		panic("unexpected default value type: SchemaElements")
	}
	if _, ok := value.(*schema.Resource); ok {
		panic("unexpected default value type: SchemaInfo")
	}
	if _, ok := value.(schema.ValueType); ok {
		panic("unexpected default value type: ValueType")
	}
	return &SchemaElement{Type: fmt.Sprintf("%T", value), Value: fmt.Sprintf("%v", value)}
}

func Generate(provider *schema.Provider, name string, outputPath string) {
	outputFilePath := filepath.Join(outputPath, name+`.json`)

	if err := DoGenerate(provider, outputFilePath); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(255)
	}
}

func DoGenerate(provider *schema.Provider, outputFilePath string) error {
	providerJson, err := json.MarshalIndent(Export(provider), "", "  ")

	if err != nil {
		return err
	}

	file, err := os.Create(outputFilePath)
	if err != nil {
		return err
	}

	//noinspection GoUnhandledErrorResult
	defer file.Close()

	_, err = file.Write(providerJson)
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
type SchemaInfoWithTimeouts map[string]interface{}

//{
//	SchemaInfo `json:""`
//	Timeouts []string `json:"__timeouts__,omitempty"`
//}

// ResourceProviderSchema
type ResourceProviderSchema struct {
	SchemaVersion string `json:".schema_version"`
	SDKType       string `json:".sdk_type"`

	Name        string                            `json:"name"`
	Type        string                            `json:"type"`
	Version     string                            `json:"version"`
	Provider    SchemaInfo                        `json:"provider"`
	Resources   map[string]SchemaInfoWithTimeouts `json:"resources"`
	DataSources map[string]SchemaInfoWithTimeouts `json:"data-sources"`
}

func main() {
	var provider tf.ResourceProvider
	//noinspection GoUnresolvedReference
	provider = prvdr.__PROVIDER_FUNC_NAME__(__PROVIDER_ARGS__)
	Generate(provider.(*schema.Provider), "__NAME__", "__OUT__")
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
	//noinspection GoVetUnsafePointer
	return *(*uintptr)(unsafe.Pointer(location))
}

func getString(addr uintptr) string {
	//noinspection GoVetUnsafePointer
	SH := (*reflect.StringHeader)(unsafe.Pointer(addr))

	var res string
	pString := (*reflect.StringHeader)(unsafe.Pointer(&res))

	pString.Data = SH.Data
	pString.Len = SH.Len
	return res
}

func getSlice(addr uintptr) []string {
	//noinspection GoVetUnsafePointer
	SH := (*reflect.SliceHeader)(unsafe.Pointer(addr))

	var res = make([]string, 3)
	pString := (*reflect.SliceHeader)(unsafe.Pointer(&res))

	pString.Data = SH.Data
	pString.Len = SH.Len
	pString.Cap = SH.Len
	return res
}
