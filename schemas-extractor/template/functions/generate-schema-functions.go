package main

//noinspection GoUnresolvedReference
import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/hcl/v2/ext/customdecode"
	"github.com/hashicorp/hil"
	"github.com/hashicorp/hil/ast"
	"github.com/hashicorp/terraform/internal/lang"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"unsafe"
)

// ExportSchema should be called to export the structure
// of the provisioner.
func Export() *FunctionsSchema {
	result := new(FunctionsSchema)
	result.SchemaVersion = "2"
	result.SDKType = "__SDK__"

	result.Name = "functions"
	result.Type = "functions"
	result.Version = "__REVISION__"
	result.Functions = getInterpolationFunctions()

	return result
}

var staticReturnTypeFun = function.StaticReturnType(cty.Bool)

func getInterpolationFunctions() map[string]FunctionInfo {
	vars := make(map[string]ast.Variable)
	cfg := getLangEvalConfig(vars)
	result := make(map[string]FunctionInfo)
	// TF 0.11
	for name, fun := range cfg.GlobalScope.FuncMap {
		args := make([]string, len(fun.ArgTypes))
		for i, at := range fun.ArgTypes {
			args[i] = shortenType(at.String())
		}
		vt := ""
		if fun.Variadic {
			vt = shortenType(fun.VariadicType.String())
		}
		result[name] = FunctionInfo{
			ArgTypes:     &args,
			ReturnType:   shortenType(fun.ReturnType.String()),
			VariadicType: vt,
		}
	}
	// TF 0.12
	scope := lang.Scope{
		PureOnly: false,
	}
	functions := scope.Functions()
	for name, f := range functions {
		if name == "map" {
			continue
		}
		if name == "list" {
			continue
		}
		params := f.Params()
		args := make([]ParameterInfo, len(params))
		types := make([]cty.Type, len(params))
		for i, at := range params {
			args[i] = *exportParameter(&at)
			types[i] = at.Type
		}
		var varParameter *ParameterInfo = nil
		if variadic := f.VarParam(); variadic != nil {
			varParameter = exportParameter(variadic)
		}

		spec := getFunctionSpec(f)
		var returnType cty.Type
		if reflect.ValueOf(spec.Type).Pointer() == reflect.ValueOf(staticReturnTypeFun).Pointer() {
			// StaticReturnType used as spec.Type
			returnType, _ = spec.Type([]cty.Value{})
		} else {
			var err error
			returnType, err = f.ReturnType(types)
			if err != nil {
				if name == "try" {
					returnType = f.VarParam().Type
				} else if f.VarParam() != nil {
					returnType, err = f.ReturnType([]cty.Type{f.VarParam().Type, f.VarParam().Type})
					if err != nil {
						panic(fmt.Errorf("failed to get return type for '%s': %v", name, err))
					}
				}
			}
		}

		info := FunctionInfo{
			Parameters:        &args,
			ReturnType:        exportType(returnType),
			VariadicParameter: varParameter,
		}
		if len(*info.Parameters) == 0 {
			info.Parameters = nil
		}
		result[name] = info
	}

	return result
}

func getFunctionSpec(f function.Function) *function.Spec {
	expected := reflect.TypeOf(&function.Spec{})
	v := reflect.Indirect(reflect.ValueOf(f))
	tp := v.Type()
	for i := 0; i < tp.NumField(); i++ {
		field := tp.Field(i)
		if field.Type == expected {
			value := v.Field(i)
			//noinspection GoVetUnsafePointer
			spec := (*function.Spec)(unsafe.Pointer(value.Pointer()))
			return spec
		}
	}
	return nil
}

func exportParameter(variadic *function.Parameter) *ParameterInfo {
	info := ParameterInfo{
		Name:             variadic.Name,
		Type:             exportType(variadic.Type),
		AllowNull:        variadic.AllowNull,
		AllowUnknown:     variadic.AllowUnknown,
		AllowDynamicType: variadic.AllowDynamicType,
		AllowMarked:      variadic.AllowMarked,
	}
	return &info
}

func getLangEvalConfig(vars map[string]ast.Variable) hil.EvalConfig {
	return hil.EvalConfig{
		GlobalScope: &ast.BasicScope{
			VarMap: vars,
		},
	}
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

func Generate(name string, outputPath string) {
	outputFilePath := filepath.Join(outputPath, name+`.json`)

	if err := DoGenerate(outputFilePath); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(255)
	}
}

func DoGenerate(outputFilePath string) error {
	functionsJson, err := json.MarshalIndent(Export(), "", "  ")

	if err != nil {
		return err
	}

	file, err := os.Create(outputFilePath)
	if err != nil {
		return err
	}

	//noinspection GoUnhandledErrorResult
	defer file.Close()

	_, err = file.Write(functionsJson)
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

type FunctionInfo struct {
	Parameters        *[]ParameterInfo `json:",omitempty"`
	ArgTypes          *[]string        `json:",omitempty"`
	ReturnType        string
	VariadicType      string         `json:",omitempty"`
	VariadicParameter *ParameterInfo `json:",omitempty"`
}

type ParameterInfo struct {
	Name             string
	Type             string
	AllowNull        bool `json:",omitempty"`
	AllowUnknown     bool `json:",omitempty"`
	AllowDynamicType bool `json:",omitempty"`
	AllowMarked      bool `json:",omitempty"`
}

// FunctionsSchema
type FunctionsSchema struct {
	SchemaVersion string `json:".schema_version"`
	SDKType       string `json:".sdk_type"`

	Name      string                  `json:"name"`
	Type      string                  `json:"type"`
	Version   string                  `json:"version"`
	Functions map[string]FunctionInfo `json:"schema"`
}

func main() {
	Generate("functions", "__OUT__")
}
