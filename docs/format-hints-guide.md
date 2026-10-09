# Format Hints Guide for Forward-MCP Tools

**Purpose:** Document correct format hints for MCP tool parameters based on Forward API spec.

**Source:** https://docs.fwd.app/latest/api/spec/complete.yaml

---

## Forward API ID Formats

### Network ID
- **Type:** string
- **Format:** Numeric string
- **Example:** `'123'`
- **JSON Schema:** `type: string` (no special format)
- **Correct hint:** None needed (plain string)
- **Incorrect:** ~~`format=uuid`~~

### Snapshot ID
- **Type:** string  
- **Format:** Numeric string or timestamp-based
- **Example:** Not specified in API spec
- **JSON Schema:** `type: string` (no special format)
- **Correct hint:** None needed (plain string)
- **Incorrect:** ~~`format=uuid`~~

### Query ID
- **Type:** string
- **Format:** Path-like identifier
- **Example:** `"devices"`, `"/L3/Basic/devices"`
- **JSON Schema:** `type: string`
- **Correct hint:** None needed (plain string)

---

## When to Use Format Hints

### IP Addresses

**Use:** `format=ipv4` or `format=ipv6`

**Example:**
```go
SrcIP string `json:"src_ip" jsonschema:"Source IP address or CIDR.;format=ipv4"`
DstIP string `json:"dst_ip" jsonschema:"Destination IP address. Required.;format=ipv4"`
```

**Why:** Helps agents generate valid IP addresses (192.168.1.1) instead of strings or device names.

### Dates and Times

**Use:** `format=date-time`

**Example:**
```go
Timestamp string `json:"timestamp" jsonschema:"ISO 8601 timestamp.;format=date-time"`
```

**Why:** Agents know to generate ISO 8601 format (2026-10-09T14:30:00Z).

### Email Addresses

**Use:** `format=email`

**Example:**
```go
Email string `json:"email" jsonschema:"Contact email address.;format=email"`
```

**Why:** Agents generate valid email format (user@example.com).

### URIs/URLs

**Use:** `format=uri` or `format=url`

**Example:**
```go
BaseURL string `json:"base_url" jsonschema:"API base URL.;format=url"`
```

---

## When NOT to Use Format Hints

### Generic Strings

**Do NOT use:** ~~`format=string`~~ (redundant)

**Example:**
```go
// Correct
Name string `json:"name" jsonschema:"Network name. Required."`

// Incorrect
Name string `json:"name" jsonschema:"Network name. Required.;format=string"`
```

### Numeric IDs

Forward API uses numeric strings for IDs, but they're just strings:

**Do NOT use:** ~~`format=uuid`~~ or ~~`format=number`~~

**Example:**
```go
// Correct
NetworkID string `json:"network_id" jsonschema:"Network ID. Required."`

// Incorrect  
NetworkID string `json:"network_id" jsonschema:"Network ID. Required.;format=uuid"`
```

### Pattern-Based Strings

Configuration patterns, device filters, search terms:

**Do NOT use:** Format hints (these are freeform)

**Example:**
```go
// Correct
Pattern string `json:"pattern" jsonschema:"Configuration pattern. Supports hierarchical format with indentation."`

// Incorrect
Pattern string `json:"pattern" jsonschema:"Configuration pattern.;format=pattern"`
```

---

## Format Hint Syntax

**JSONSchema tag format:**
```go
Field Type `json:"json_name" jsonschema:"Description text.;format=format_name"`
```

**Note:** Use semicolon (`;`) to separate format from description.

**Multiple hints:**
```go
// This is valid
Field string `json:"field" jsonschema:"Description.;format=email;pattern=^[a-z]+@"`

// But keep it simple - usually one hint is enough
```

---

## Current Forward-MCP Usage

### Phase 1 + 2 (15 tools)

**IP address fields:**
- ✅ `SrcIP`, `DstIP` in path search tools → Should have `format=ipv4`
- ⚠️ Not yet added (to be done in Phase 3)

**Network/Snapshot IDs:**
- ✅ Correctly described as "Network ID" / "Snapshot ID"
- ✅ No format hints (correct - they're plain strings)

**Query IDs:**
- ✅ Correctly described as strings
- ✅ No format hints (correct - they're path identifiers)

---

## Action Items

### Phase 3 Fixes

When completing remaining 42 tools:

1. **Add IP format hints** to path search parameters:
   ```go
   SrcIP string `json:"src_ip,omitempty" jsonschema:"Source IP address or CIDR.;format=ipv4"`
   DstIP string `json:"dst_ip" jsonschema:"Destination IP address. Required.;format=ipv4"`
   ```

2. **Keep ID fields as plain strings** (no format hints):
   ```go
   NetworkID string `json:"network_id" jsonschema:"Network ID. Required."`
   SnapshotID string `json:"snapshot_id,omitempty" jsonschema:"Snapshot ID. Optional."`
   ```

3. **Add date-time hints** if we have timestamp fields:
   ```go
   CreatedAt string `json:"created_at" jsonschema:"Creation timestamp.;format=date-time"`
   ```

### Validation

Before Phase 3 commit:
- [ ] Verify no `format=uuid` in code
- [ ] Verify IP fields have `format=ipv4`  
- [ ] Test with real agent to see if hints improve parameter generation

---

## References

- Forward API Spec: https://docs.fwd.app/latest/api/spec/complete.yaml
- JSON Schema Format: https://json-schema.org/understanding-json-schema/reference/string#format
- ADR-2610091555: Tool Quality Standards
- Composio Guide: https://composio.dev/blog/how-to-build-tools-for-ai-agents-a-field-guide
