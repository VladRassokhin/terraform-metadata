package main

//noinspection GoUnresolvedReference
import (
	"github.com/hashicorp/hcl/v2/ext/customdecode"
	"github.com/zclconf/go-cty/cty"
	cty_json "github.com/zclconf/go-cty/cty/json"

	prvdr "__PKG_PREFIX__/__PKG_NAME__"
	"__PKG_PREFIX__/tfplugin5"

	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ExportSchema should be called to export the structure
// of the provider.
func Export(p *tfplugin5.GetProviderSchema_Response) *ResourceProviderSchema {
	result := new(ResourceProviderSchema)
	result.SchemaVersion = "2"
	result.SDKType = "terraform-tfplugin5"

	result.Name = "__NAME__"
	result.Type = "provider"
	result.Version = "__REVISION__"
	result.Provider = ExportSchema(p.Provider)
	result.Resources = make(map[string]SchemaInfo)
	result.DataSources = make(map[string]SchemaInfo)

	for k, r := range p.ResourceSchemas {
		result.Resources[k] = ExportSchema(r)
	}
	for k, ds := range p.DataSourceSchemas {
		result.DataSources[k] = ExportSchema(ds)
	}

	return result
}

//region helpers for new cty.Type schemas
func exportAttribute(v *tfplugin5.Schema_Attribute) SchemaDefinition {
	item := SchemaDefinition{}

	unmarshalType, _ := cty_json.UnmarshalType(v.Type)
	item.Type = exportType(unmarshalType)
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
	if t.IsObjectType() {
		attributeTypes := t.AttributeTypes()
		names := make([]string, 0)
		for k, _ := range attributeTypes {
			names = append(names, k)
		}
		sort.Strings(names)
		ret := "Object({"
		first := true
		for _, name := range names {
			if first {
				first = true
			} else {
				ret += ","
			}
			ret += " " + name + "=" + exportType(attributeTypes[name])
		}
		if len(names) > 0 {
			ret += " "
		}
		ret += "})"
		return ret
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

func exportBlock(block *tfplugin5.Schema_Block) SchemaInfo {
	result := make(SchemaInfo)
	for _, attr := range block.Attributes {
		result[attr.Name] = exportAttribute(attr)
	}
	for _, blk := range block.BlockTypes {
		result[blk.TypeName] = exportNestedBlock(blk)
	}
	return result
}

func exportNestedBlock(v *tfplugin5.Schema_NestedBlock) SchemaDefinition {
	item := SchemaDefinition{}

	item.MinItems = v.MinItems
	item.MaxItems = v.MaxItems
	switch v.Nesting {
	case tfplugin5.Schema_NestedBlock_SINGLE:
		item.Type = "NestingSingle"
	case tfplugin5.Schema_NestedBlock_GROUP:
		item.Type = "NestingGroup"
	case tfplugin5.Schema_NestedBlock_LIST:
		item.Type = "NestingList"
	case tfplugin5.Schema_NestedBlock_SET:
		item.Type = "NestingSet"
	case tfplugin5.Schema_NestedBlock_MAP:
		item.Type = "NestingMap"
	}

	item.Elem = &SchemaElement{Type: "SchemaInfo", Info: exportBlock(v.Block)}

	item.IsBlock = true
	item.ConfigImplicitMode = "Block"
	return item
}

//endregion

/*
func (m schemaMap) Export() SchemaInfo {
	result := make(SchemaInfo)
	for k, v := range m {
		item := export(v)
		result[k] = item
	}
	return result
}

*/
func ExportSchema(m *tfplugin5.Schema) SchemaInfo {
	return exportBlock(m.Block)
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

func Generate(provider tfplugin5.ProviderServer, name string, outputPath string) {
	outputFilePath := filepath.Join(outputPath, name+`.json`)

	if err := DoGenerate(provider, outputFilePath); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(255)
	}
}

func DoGenerate(provider tfplugin5.ProviderServer, outputFilePath string) error {
	schemaResponse, err := (provider).GetSchema(nil, nil)
	if err != nil {
		return err
	}
	providerJson, err := json.MarshalIndent(Export(schemaResponse), "", "  ")

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
	MaxItems      int64  `json:",omitempty"`
	MinItems      int64  `json:",omitempty"`
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

// ResourceProviderSchema
type ResourceProviderSchema struct {
	SchemaVersion string `json:".schema_version"`
	SDKType       string `json:".sdk_type"`

	Name        string                `json:"name"`
	Type        string                `json:"type"`
	Version     string                `json:"version"`
	Provider    SchemaInfo            `json:"provider"`
	Resources   map[string]SchemaInfo `json:"resources"`
	DataSources map[string]SchemaInfo `json:"data-sources"`
}

func main() {
	var provider tfplugin5.ProviderServer
	//noinspection GoUnresolvedReference
	provider = &prvdr.RawProviderServer{}
	Generate(provider, "__NAME__", "__OUT__")
}
