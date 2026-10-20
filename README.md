# kiln-core

The Go packages shared by the [Kiln](https://github.com/pixelpolishers/kiln-cli) command line tool and the Kiln registry. Kiln is a package manager for C++.

| Package | What it does |
|---|---|
| `semver` | Parse, compare and order versions |
| `constraint` | Parse and evaluate version constraints: exact versions, wildcards (`3.*`), caret (`^3.12`), tilde (`~3.12.1`), comparison operators, `AND`/`OR` and ranges |
| `solver` | Pick one version of every package so that all constraints hold, with an explanation when that is impossible |
| `options` | Resolve package options that several dependents ask for, and refuse conflicting requests |
| `assetid` | Compute the identity of a built binary from the package, version, build settings, options and the builds of its dependencies |
| `when` | Evaluate the `when` conditions of a manifest (`os`, `arch`, `compiler`, `build_type`, `options.<name>`, ...) |
| `manifest` | Typed `kiln.json`, asset list and package description, and validation against the JSON schemas |
| `api` | The data types of the registry's HTTP API |

The code has no dependencies besides the standard library and a JSON Schema validator, and does not touch the network or the file system.

## Use

```
go get github.com/pixelpolishers/kiln-core
```

```go
v, _ := semver.Parse("3.12.0")
c, _ := constraint.Parse("^3.10")
fmt.Println(c.Matches(v)) // true
```

The JSON schemas live in [kiln-schema](https://github.com/pixelpolishers/kiln-schema), which `manifest` depends on. The repositories are not versioned yet: until they are tagged, develop against a clone of each in a Go workspace (`go work init ./kiln-core ./kiln-schema`).

## Develop

```
go test ./...
```

## Status

Kiln is a working prototype; the packages and their APIs may still change.

## License

MIT, see [LICENSE](LICENSE).
