package main

import (
	pkg "__REPOSITORY__/__PKG_NAME__"
	"github.com/hashicorp/hcl/v2/ext/customdecode"
	"github.com/hashicorp/terraform/internal/configs/configschema"
	"github.com/hashicorp/terraform/internal/provisioners"
	"github.com/zclconf/go-cty/cty"

	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Export should be called to export the structure of the provisioner.
func Export(p *configschema.Block) *ResourceProvisionerSchema {
	result := new(ResourceProvisionerSchema)
	result.SchemaVersion = "2"
	result.SDKType = "__SDK__"

	result.Name = "__NAME__"
	result.Type = "provisioner"
	result.Version = "__REVISION__"
	result.Provisioner = exportBlock(p)

	return result
}

func shortenType(value string) string {
	if len(value) > 4 && value[0:4] == "Type" {
		return value[4:]
	}
	return value
}

func Generate(provisioner provisioners.Interface, name string, outputPath string) {
	outputFilePath := filepath.Join(outputPath, name+`.json`)
	p := provisioner.GetSchema().Provisioner

	if err := DoGenerate(p, outputFilePath); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(255)
	}
}

func DoGenerate(provisioner *configschema.Block, outputFilePath string) error {
	provisionerJson, err := json.MarshalIndent(Export(provisioner), "", "  ")

	if err != nil {
		return err
	}

	file, err := os.Create(outputFilePath)
	if err != nil {
		return err
	}

	//goland:noinspection GoUnhandledErrorResult
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

type ResourceProvisionerSchema struct {
	SchemaVersion string `json:".schema_version"`
	SDKType       string `json:".sdk_type"`

	Name        string     `json:"name"`
	Type        string     `json:"type"`
	Version     string     `json:"version"`
	Provisioner SchemaInfo `json:"schema"`
}

func main() {
	var provisioner provisioners.Interface
	provisioner = pkg.New()
	Generate(provisioner, "__NAME__", "__OUT__")
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
