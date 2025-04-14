# ModCLI

ModCLI is a command-line interface tool for the [Modular](https://github.com/GoCodeAlone/modular) framework that helps you scaffold and generate code for modular applications.

## Installation

### Using Go Install (Recommended)

Install the latest version directly using Go:

```bash
go install github.com/GoCodeAlone/modular/cmd/modcli@latest
```

After installation, the `modcli` command will be available in your PATH.

### From Source

```bash
git clone https://github.com/GoCodeAlone/modular.git
cd modular/cmd/modcli
go install
```

### From Releases

Download the latest release for your platform from the [releases page](https://github.com/GoCodeAlone/modular/releases).

## Commands

ModCLI provides several commands to help you build modular applications:

### Generate Module

Create a new module for your modular application with the following command:

```bash
modcli generate module --name MyModule --output ./modules
```

This will create a new module in the specified output directory with the given name. The command will prompt you for module features including:

- Configuration support
- Tenant awareness
- Module dependencies
- Startup and shutdown logic
- Service provisioning
- Test generation

For configuration-enabled modules, you can define configuration fields, types, and validation requirements.

### Generate Config

Create a new configuration structure with:

```bash
modcli generate config --name AppConfig --output ./config
```

This command helps you define configuration structures with proper validation, default values, and serialization formats (YAML, JSON, TOML, etc.).

## Examples

### Creating a Basic Module

```bash
# Generate a basic module with minimal features
modcli generate module --name Basic --output .
```

### Creating a Full-Featured Module

```bash
# Generate a module with all features enabled
modcli generate module --name FullFeatured --output .
```

When prompted, select all features (configuration, tenant awareness, etc.)

### Creating a Configuration-Only Module

```bash
# Generate a module that focuses on configuration
modcli generate module --name ConfigOnly --output .
```

When prompted, select configuration support but disable other features.

## Generated Files

### For Modules

The `generate module` command creates the following files:

- `module.go` - Main module implementation
- `config.go` - Configuration structure (if enabled)
- `config-sample.yaml/json/toml` - Sample configuration files (if enabled)
- `module_test.go` - Test file with test cases for your module
- `mock_test.go` - Mock implementations for testing
- `README.md` - Documentation for your module
- `go.mod` - Go module file

### For Config

The `generate config` command creates:

- `config.go` - Configuration structure with validation
- `config-sample.yaml/json/toml` - Sample configuration files

## Development

ModCLI is built using the [cobra](https://github.com/spf13/cobra) command framework and generates code using Go templates.

### Project Structure

- `main.go` - Entry point
- `cmd/` - Command implementations
  - `root.go` - Root command
  - `generate_module.go` - Module generation logic
  - `generate_config.go` - Config generation logic

### Running Tests

```bash
go test ./... -v
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.