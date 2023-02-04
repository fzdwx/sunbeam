package app

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/santhosh-tekuri/jsonschema/v5"
	_ "github.com/santhosh-tekuri/jsonschema/v5/httploader"
	"gopkg.in/yaml.v3"
)

//go:embed schemas
var embedFs embed.FS

type RootItem struct {
	Extension string                  `json:"extension,omitempty" yaml:"extension,omitempty"`
	Command   string                  `json:"command,omitempty" yaml:"command,omitempty"`
	Title     string                  `json:"title,omitempty" yaml:"title,omitempty"`
	With      map[string]CommandInput `json:"with,omitempty" yaml:"with,omitempty"`
}

type Extension struct {
	Version     string               `json:"version" yaml:"version"`
	Title       string               `json:"title" yaml:"title"`
	Description string               `json:"description,omitempty" yaml:"description,omitempty"`
	Platform    []string             `json:"platform,omitempty" yaml:"platform,omitempty"`
	PostInstall string               `json:"postInstall,omitempty" yaml:"postInstall,omitempty"`
	Root        string               `json:"-" yaml:"-"`
	Preferences map[string]*FormItem `json:"preferences,omitempty" yaml:"preferences,omitempty"`

	Requirements []ExtensionRequirement `json:"requirements,omitempty" yaml:"requirements,omitempty"`
	RootItems    []RootItem             `json:"rootItems" yaml:"rootItems"`
	Commands     []Command              `json:"commands"`
}

func (e Extension) Name() string {
	return path.Base(e.Root)
}

func (e Extension) GetCommand(name string) (Command, bool) {
	for _, command := range e.Commands {
		if command.Name == name {
			return command, true
		}
	}
	return Command{}, false
}

type ExtensionRequirement struct {
	Which    string
	HomePage string `json:"homePage" yaml:"homePage"`
}

func (r ExtensionRequirement) Check() bool {
	if _, err := exec.LookPath(r.Which); err != nil {
		return false
	}
	return true
}

var ExtensionSchema *jsonschema.Schema
var PageSchema *jsonschema.Schema

func init() {
	var err error

	compiler := jsonschema.NewCompiler()

	manifest, err := embedFs.Open("schemas/extension.json")
	if err != nil {
		panic(err)
	}
	if err = compiler.AddResource("https://pomdtr.github.io/sunbeam/schemas/extension.json", manifest); err != nil {
		panic(err)
	}

	page, err := embedFs.Open("schemas/page.json")
	if err != nil {
		panic(err)
	}
	if err = compiler.AddResource("https://pomdtr.github.io/sunbeam/schemas/page.json", page); err != nil {
		panic(err)
	}

	ExtensionSchema, err = compiler.Compile("https://pomdtr.github.io/sunbeam/schemas/extension.json")
	if err != nil {
		panic(err)
	}

	PageSchema, err = compiler.Compile("https://pomdtr.github.io/sunbeam/schemas/page.json")
	if err != nil {
		panic(err)
	}
}

func LoadExtensions(extensionRoot string) ([]*Extension, error) {
	entries, err := os.ReadDir(extensionRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to read extension root: %w", err)
	}

	extensions := make([]*Extension, 0)
	for _, entry := range entries {
		extensionDir := path.Join(extensionRoot, entry.Name())
		if fi, err := os.Stat(extensionDir); err != nil || !fi.IsDir() {
			continue
		}

		extension, err := LoadExtension(extensionDir)
		if err != nil {
			continue
		}

		extensions = append(extensions, extension)
	}

	return extensions, nil
}

func LoadExtension(extensionDir string) (*Extension, error) {
	manifestPath := path.Join(extensionDir, "sunbeam.yml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("extension manifest not found: %w", err)
	}

	extension, err := ParseManifest(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse extension manifest: %w", err)
	}
	extension.Root = extensionDir

	return &extension, nil

}

func ParseManifest(manifestPath string) (extension Extension, err error) {
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return extension, err
	}

	var m any
	err = yaml.Unmarshal(manifestBytes, &m)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return extension, err
	}

	err = ExtensionSchema.Validate(m)
	if err != nil {
		return extension, err
	}

	err = yaml.Unmarshal(manifestBytes, &extension)
	if err != nil {
		return extension, err
	}

	return extension, nil
}
