package service

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/forward-mcp/internal/forward"
	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const cdpLldpTopologyQueryID = "FQ_08cb4fd1d50cb521e25a43714e85f23c1e664b34"

type topoEdge struct {
	srcDevice string
	srcIface  string
	dstDevice string
	dstIface  string
}

func (s *ForwardMCPService) generateDrawioTopology(args GenerateDrawioTopologyArgs) (*mcp.CallToolResult, error) {
	s.logToolCall("generate_drawio_topology", args, nil)

	networkID := s.getNetworkID(args.NetworkID)
	if err := s.validateNetworkID(networkID); err != nil {
		return nil, err
	}
	snapshotID := s.getSnapshotID(args.SnapshotID)

	// Fetch all CDP/LLDP topology data with pagination
	var allItems []map[string]interface{}
	offset := 0
	const batchSize = 500
	for {
		result, err := s.forwardClient.RunNQEQueryByID(&forward.NQEQueryParams{
			NetworkID:  networkID,
			SnapshotID: snapshotID,
			QueryID:    cdpLldpTopologyQueryID,
			Options:    &forward.NQEQueryOptions{Limit: batchSize, Offset: offset},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to run L2 CDP/LLDP topology query: %w", err)
		}
		allItems = append(allItems, result.Items...)
		if len(result.Items) < batchSize {
			break
		}
		offset += batchSize
	}

	if len(allItems) == 0 {
		return newToolResponse(newTextContent(
			"No CDP/LLDP neighbor data found in this network snapshot. " +
				"Ensure CDP/LLDP is enabled on network devices.",
		)), nil
	}

	nodes, edges := parseL2Topology(allItems)

	xmlContent := buildDrawioXML(nodes, edges)

	// Save .drawio file to the Claude Desktop shared directory
	outputPath := args.OutputPath
	if outputPath == "" {
		ts := time.Now().Format("20060102-150405")
		outputPath = filepath.Join(claudeSharedDir(), fmt.Sprintf("forward-topology-%s.drawio", ts))
	}
	if err := os.WriteFile(outputPath, []byte(xmlContent), 0644); err != nil {
		s.logger.Warn("Failed to save topology file: %v", err)
	}

	// draw.io has no server-side REST API for creating editable diagrams — it is a
	// client-side app. The standard programmatic approach is to encode the XML in
	// the URL fragment, which draw.io's JavaScript reads on load via window.location.hash.
	// This is the same mechanism used by draw.io's own "Share" feature.
	drawioURL := "https://app.diagrams.net/#R" + url.QueryEscape(xmlContent)

	if err := openBrowser(drawioURL); err != nil {
		s.logger.Warn("Failed to open browser automatically: %v", err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Generated L2 CDP/LLDP topology: %d devices, %d links\n\n", len(nodes), len(edges))
	fmt.Fprintf(&sb, "Saved to: %s\n", outputPath)
	fmt.Fprintf(&sb, "Opened:   %s\n\n", drawioURL)
	sb.WriteString("Tip: In draw.io use Arrange > Apply Layout > Organic for the best view.\n")

	return newToolResponse(newTextContent(sb.String())), nil
}

// parseL2Topology extracts deduplicated device nodes and link edges from CDP/LLDP NQE rows.
// It tries multiple common column name conventions to be robust across query variations.
func parseL2Topology(items []map[string]interface{}) (nodes []string, edges []topoEdge) {
	localDeviceCols := []string{"device", "hostname", "localDevice", "deviceA", "srcDevice", "source"}
	localIfaceCols := []string{"interface", "localInterface", "port", "interfaceA", "localPort", "srcInterface"}
	remoteDeviceCols := []string{"neighborDevice", "neighbor", "neighborHostname", "deviceB", "dstDevice", "remoteDevice", "remoteHostname"}
	remoteIfaceCols := []string{"neighborInterface", "remoteInterface", "neighborPort", "interfaceB", "dstInterface", "remotePort"}

	deviceSet := make(map[string]bool)

	for _, item := range items {
		srcDev := pickFirstStr(item, localDeviceCols)
		srcIface := pickFirstStr(item, localIfaceCols)
		dstDev := pickFirstStr(item, remoteDeviceCols)
		dstIface := pickFirstStr(item, remoteIfaceCols)

		if srcDev == "" || dstDev == "" {
			continue
		}

		deviceSet[srcDev] = true
		deviceSet[dstDev] = true
		edges = append(edges, topoEdge{
			srcDevice: srcDev,
			srcIface:  srcIface,
			dstDevice: dstDev,
			dstIface:  dstIface,
		})
	}

	for dev := range deviceSet {
		nodes = append(nodes, dev)
	}
	sort.Strings(nodes)

	// CDP/LLDP reports each physical link from both directions; keep one copy.
	edges = deduplicateEdges(edges)

	return nodes, edges
}

func deduplicateEdges(edges []topoEdge) []topoEdge {
	seen := make(map[string]bool, len(edges))
	out := make([]topoEdge, 0, len(edges)/2+1)
	for _, e := range edges {
		key := canonicalEdgeKey(e)
		if !seen[key] {
			seen[key] = true
			out = append(out, e)
		}
	}
	return out
}

// canonicalEdgeKey produces a direction-independent key for a link.
func canonicalEdgeKey(e topoEdge) string {
	a := e.srcDevice + ":" + e.srcIface
	b := e.dstDevice + ":" + e.dstIface
	if a > b {
		a, b = b, a
	}
	return a + "|" + b
}

// pickFirstStr returns the string value of the first matching key found in m.
func pickFirstStr(m map[string]interface{}, keys []string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			s := fmt.Sprintf("%v", v)
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

// buildDrawioXML generates mxGraph XML suitable for import into draw.io.
func buildDrawioXML(nodes []string, edges []topoEdge) string {
	const (
		nodeW    = 120
		nodeH    = 60
		hSpacing = 200
		vSpacing = 140
	)

	cols := int(math.Ceil(math.Sqrt(float64(len(nodes)))))
	if cols < 1 {
		cols = 1
	}

	nodeID := make(map[string]string, len(nodes))

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sb.WriteString(`<mxfile version="21.0.0">` + "\n")
	sb.WriteString(`  <diagram id="topology" name="L2 CDP/LLDP Topology">` + "\n")
	sb.WriteString(`    <mxGraphModel dx="1422" dy="762" grid="1" gridSize="10" guides="1" tooltips="1" connect="1" arrows="1" fold="1" page="1" pageScale="1" pageWidth="1654" pageHeight="1169" math="0" shadow="0">` + "\n")
	sb.WriteString(`      <root>` + "\n")
	sb.WriteString(`        <mxCell id="0"/>` + "\n")
	sb.WriteString(`        <mxCell id="1" parent="0"/>` + "\n")

	for i, node := range nodes {
		id := fmt.Sprintf("d%d", i+2)
		nodeID[node] = id
		col := i % cols
		row := i / cols
		x := col*hSpacing + 50
		y := row*vSpacing + 50

		sb.WriteString(fmt.Sprintf(
			`        <mxCell id=%q value=%q style="rounded=1;whiteSpace=wrap;html=1;fillColor=#dae8fc;strokeColor=#6c8ebf;fontStyle=1;fontSize=11;" vertex="1" parent="1">`+"\n",
			id, drawioEscape(node),
		))
		sb.WriteString(fmt.Sprintf(
			`          <mxGeometry x="%d" y="%d" width="%d" height="%d" as="geometry"/>`+"\n",
			x, y, nodeW, nodeH,
		))
		sb.WriteString(`        </mxCell>` + "\n")
	}

	for i, e := range edges {
		srcID, ok1 := nodeID[e.srcDevice]
		dstID, ok2 := nodeID[e.dstDevice]
		if !ok1 || !ok2 {
			continue
		}
		edgeID := fmt.Sprintf("e%d", len(nodes)+2+i)

		label := ""
		if e.srcIface != "" || e.dstIface != "" {
			label = drawioEscape(e.srcIface + " — " + e.dstIface)
		}

		sb.WriteString(fmt.Sprintf(
			`        <mxCell id=%q value=%q style="edgeStyle=elbowEdgeStyle;html=1;fontSize=9;" edge="1" source=%q target=%q parent="1">`+"\n",
			edgeID, label, srcID, dstID,
		))
		sb.WriteString(`          <mxGeometry relative="1" as="geometry"/>` + "\n")
		sb.WriteString(`        </mxCell>` + "\n")
	}

	sb.WriteString(`      </root>` + "\n")
	sb.WriteString(`    </mxGraphModel>` + "\n")
	sb.WriteString(`  </diagram>` + "\n")
	sb.WriteString(`</mxfile>` + "\n")

	return sb.String()
}

// drawioEscape escapes special characters for use as an XML attribute value.
func drawioEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// claudeSharedDir returns the coworkUserFilesPath from the Claude Desktop
// config, falling back to ~/Downloads if the config is absent or unparseable.
func claudeSharedDir() string {
	home, _ := os.UserHomeDir()
	fallback := filepath.Join(home, "Downloads")

	cfgPath := filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return fallback
	}

	var cfg struct {
		CoworkUserFilesPath string `json:"coworkUserFilesPath"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil || cfg.CoworkUserFilesPath == "" {
		return fallback
	}

	_ = os.MkdirAll(cfg.CoworkUserFilesPath, 0755)
	return cfg.CoworkUserFilesPath
}

// openBrowser opens url in the default system browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	return cmd.Start()
}
