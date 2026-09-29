# Task 2 Report: How Secrets flow through SiteState

## Data flow summary

```
Secret-<name>.yaml on disk
  └─ FileSystemSiteStateLoader.Load()              # site_state_loader.go:30
       └─ LoadIntoSiteState(reader, siteState)      # site_state_loader.go:96
            └─ yamlDecoder.Decode(&rawObj)           # generic YAML decode
                 └─ DefaultUnstructuredConverter.FromUnstructured → corev1.Secret
                      └─ siteState.Secrets[secret.Name] = &secret   # line 170

MarshalSiteState(siteState, outputDir)              # site_state.go:449
  └─ marshalMap(outputDir, "Secret", siteState.Secrets)  # line 484
       └─ marshal(outputDir, "Secret", name, secret)     # line 442
            └─ yaml.Encode(secret, file)            # writes Secret-<name>.yaml
```

## Step-by-step trace

### 1. `SiteState` struct — `pkg/nonkube/api/site_state.go:30`

```go
type SiteState struct {
    // ...
    Secrets    map[string]*corev1.Secret    // line 42
    // ...
}
```

Initialized in `NewSiteState()` (line 47) as `make(map[string]*corev1.Secret)` (line 59). Uses the standard K8s `corev1.Secret` type with no Skupper-specific wrapper.

### 2. `FileSystemSiteStateLoader.Load()` — `internal/nonkube/common/site_state_loader.go:30`

Reads all `.yaml`/`.yml` files from the loader's `Path` directory. For each file, it opens a buffered reader and calls `LoadIntoSiteState(reader, siteState)` (line 47). There is no filename-based filtering — `Secret-*.yaml` is a naming convention, not a dispatch mechanism. Any YAML file in the directory is decoded and routed by its GVK.

### 3. `LoadIntoSiteState()` — `internal/nonkube/common/site_state_loader.go:96`

This is the core routing function. It supports multi-document YAML (loops until EOF). For each document:

1. **Raw decode**: `yamlDecoder.Decode(&rawObj)` (line 106)
2. **GVK extraction**: `yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)` (line 114) — returns the unstructured object and its GroupVersionKind
3. **GVK routing**: Two top-level branches:
   - `v2alpha1.SchemeGroupVersion` (line 119): Skupper CRDs (Site, Listener, Connector, Link, etc.)
   - `corev1.SchemeGroupVersion` (line 165): K8s core types (Secret, ConfigMap)

For Secrets (lines 167-170):

```go
case "Secret":
    var secret corev1.Secret
    runtime.DefaultUnstructuredConverter.FromUnstructured(
        obj.(runtime.Unstructured).UnstructuredContent(), &secret)
    siteState.Secrets[secret.Name] = &secret
```

**No type-based filtering occurs.** The loader does not inspect `secret.Type`. A `kubernetes.io/basic-auth` Secret, an `Opaque` Secret, and a TLS Secret all flow through the identical code path. The Secret's `Type` field is preserved on the struct but is not used as a discriminator during loading.

### 4. `MarshalSiteState()` — `pkg/nonkube/api/site_state.go:449`

Serializes the full SiteState back to YAML files. Line 484:

```go
if err = marshalMap(outputDirectory, "Secret", siteState.Secrets); err != nil {
    return err
}
```

`marshalMap()` (line 439) iterates the map and calls `marshal()` (line 419) for each entry, which:
1. Creates the output directory (`os.MkdirAll`, line 421)
2. Creates a file named `Secret-<name>.yaml` (line 425)
3. Encodes via `json.NewYAMLSerializer` (line 431-432)

This round-trips the Secret back to disk in the same format.

## Verification: `kubernetes.io/basic-auth` Secret with `stringData`

Traced the code path for a YAML file like:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: test-proxy
type: kubernetes.io/basic-auth
stringData:
  host: "proxy.example.com"
  port: "3128"
  username: "proxyuser"
  password: "proxypass"
```

**Result**: The loading pipeline handles this without special-casing. `siteState.Secrets["test-proxy"]` will be populated with:
- `secret.Type` = `"kubernetes.io/basic-auth"`
- `secret.Name` = `"test-proxy"`

**Critical finding on `stringData` vs `Data`**: When loaded via `DefaultUnstructuredConverter.FromUnstructured` (which is what the non-kube loader uses), `stringData` fields populate `secret.StringData`, NOT `secret.Data`. This differs from K8s API server behavior, where `stringData` is merged into `Data` on write and never appears in reads.

Verified empirically:
- YAML with `stringData:` section → `secret.StringData["host"]` is populated, `secret.Data` is empty
- YAML with `data:` section (base64-encoded values) → `secret.Data["host"]` is populated, `secret.StringData` is empty

The K8s proxy code (`internal/kube/site/site.go:1316-1319`) reads from `secret.Data["host"]`, `secret.Data["port"]`, etc. If the non-kube implementation reads the same field, Secrets using `stringData` in their YAML will silently produce empty proxy configs.

## Key observations for proxy implementation

1. **No new parsing code needed.** The generic `LoadIntoSiteState()` Secret path (line 167-170) handles proxy Secrets out of the box. The discriminator for identifying proxy Secrets is `secret.Type == "kubernetes.io/basic-auth"`, checked at the point of use (e.g., in `getProxyConfig()`), not at load time.

2. **`stringData` vs `Data` is a real trap.** The non-kube `getProxyConfig()` implementation must read from `secret.StringData` when `secret.Data` is empty, or (preferably) the loader/renderer must merge `StringData` into `Data` after loading to match K8s API server semantics. If the design spec recommends `stringData` in the user-facing YAML format (which is more ergonomic — no base64), this merge step is mandatory.

3. **Secret name is the map key.** `siteState.Secrets[secret.Name]` (line 170) uses the metadata `name` field as the key. The Link's `Settings["proxy-configuration"]` value must match this name exactly.

4. **Round-trip fidelity.** `MarshalSiteState` writes `Secret-<name>.yaml` files that can be re-loaded by the same pipeline. The `Type` field survives the round-trip via the K8s YAML serializer.

5. **Existing test pattern.** The test fixture (`site_state_test.go:412-422`) already demonstrates a Secret in the `Secrets` map with `Data` fields. Adding a `kubernetes.io/basic-auth` Secret to this fixture for proxy testing would follow the established pattern.
