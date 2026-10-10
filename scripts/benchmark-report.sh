#!/usr/bin/env bash
# benchmark-report.sh - Generate formatted benchmark reports
set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Configuration
BENCHTIME="${BENCHTIME:-1s}"
OUTPUT_DIR="${BENCHMARK_OUTPUT_DIR:-./benchmark-results}"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

# Create output directory
mkdir -p "$OUTPUT_DIR"

echo -e "${BOLD}${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BOLD}${CYAN}  Forward MCP Benchmark Report${NC}"
echo -e "${BOLD}${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${BLUE}Date:${NC}      $(date '+%Y-%m-%d %H:%M:%S')"
echo -e "${BLUE}Platform:${NC}  $(uname -s) $(uname -m)"
echo -e "${BLUE}Go:${NC}        $(go version | awk '{print $3}')"
echo ""

# Run benchmarks and capture output
echo -e "${YELLOW}Running benchmarks...${NC}"
echo ""

RAW_OUTPUT="$OUTPUT_DIR/raw-$TIMESTAMP.txt"
FORMATTED_OUTPUT="$OUTPUT_DIR/report-$TIMESTAMP.txt"

# Run benchmarks and save raw output (filter out INFO/DEBUG logs)
CGO_ENABLED=1 go test -bench=. -run=^$ -benchtime="$BENCHTIME" -benchmem ./internal/... 2>&1 | \
    grep -v "\[INFO\]" | \
    grep -v "\[DEBUG\]" | \
    tee "$RAW_OUTPUT"

echo ""
echo -e "${GREEN}✓ Raw results saved to: ${NC}$RAW_OUTPUT"
echo ""

# Parse and format results
echo -e "${BOLD}${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BOLD}${CYAN}  Benchmark Summary${NC}"
echo -e "${BOLD}${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Function to format time
format_time() {
    local ns=$1
    if (( ns < 1000 )); then
        echo "${ns} ns"
    elif (( ns < 1000000 )); then
        echo "$(awk "BEGIN {printf \"%.2f\", $ns/1000}") µs"
    elif (( ns < 1000000000 )); then
        echo "$(awk "BEGIN {printf \"%.2f\", $ns/1000000}") ms"
    else
        echo "$(awk "BEGIN {printf \"%.2f\", $ns/1000000000}") s"
    fi
}

# Function to format bytes
format_bytes() {
    local bytes=$1
    if (( bytes < 1024 )); then
        echo "${bytes} B"
    elif (( bytes < 1048576 )); then
        echo "$(awk "BEGIN {printf \"%.2f\", $bytes/1024}") KB"
    else
        echo "$(awk "BEGIN {printf \"%.2f\", $bytes/1048576}") MB"
    fi
}

# Parse benchmark results
{
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "AUTO-HYDRATION BENCHMARKS"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

    grep "^Benchmark" "$RAW_OUTPUT" | grep -E "DatabaseHydration|IncrementalUpdate|QueryStore|SmartCaching|CompleteAutoHydration" | while read -r line; do
        name=$(echo "$line" | awk '{print $1}' | sed 's/Benchmark//; s/-[0-9]*$//')
        iterations=$(echo "$line" | awk '{print $2}')
        ns_per_op=$(echo "$line" | awk '{print $3}')
        bytes_per_op=$(echo "$line" | awk '{print $5}')
        allocs_per_op=$(echo "$line" | awk '{print $7}')

        printf "%-40s %10s ops   %12s/op   %12s   %6s allocs\n" \
            "$name" \
            "$iterations" \
            "$(format_time ${ns_per_op%.*})" \
            "$(format_bytes ${bytes_per_op%.*})" \
            "$allocs_per_op"
    done

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "QUERY INDEX BENCHMARKS"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

    grep "^Benchmark" "$RAW_OUTPUT" | grep -E "GenerateEmbeddings|LoadFromQueries|SearchQueries" | while read -r line; do
        name=$(echo "$line" | awk '{print $1}' | sed 's/Benchmark//; s/-[0-9]*$//')
        iterations=$(echo "$line" | awk '{print $2}')
        ns_per_op=$(echo "$line" | awk '{print $3}')
        bytes_per_op=$(echo "$line" | awk '{print $5}')
        allocs_per_op=$(echo "$line" | awk '{print $7}')

        printf "%-40s %10s ops   %12s/op   %12s   %6s allocs\n" \
            "$name" \
            "$iterations" \
            "$(format_time ${ns_per_op%.*})" \
            "$(format_bytes ${bytes_per_op%.*})" \
            "$allocs_per_op"
    done

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

} | tee "$FORMATTED_OUTPUT"

# Print key insights
echo -e "${BOLD}${CYAN}Key Insights:${NC}"
echo ""

# Extract specific metrics
db_hydration=$(grep "BenchmarkDatabaseHydration-" "$RAW_OUTPUT" | awk '{print $3}' | sed 's/ ns\/op//')
complete_hydration=$(grep "BenchmarkCompleteAutoHydration-" "$RAW_OUTPUT" | awk '{print $3}' | sed 's/ ns\/op//')
bm25_search=$(grep "BenchmarkSearchQueries_BM25-" "$RAW_OUTPUT" | awk '{print $3}' | sed 's/ ns\/op//')
hybrid_search=$(grep "BenchmarkSearchQueries_Hybrid-" "$RAW_OUTPUT" | awk '{print $3}' | sed 's/ ns\/op//')

if [ -n "$complete_hydration" ]; then
    echo -e "  ${BLUE}•${NC} First-run auto-hydration: $(format_time ${complete_hydration%.*})"
fi

if [ -n "$bm25_search" ] && [ -n "$hybrid_search" ]; then
    speedup=$(awk "BEGIN {printf \"%.1f\", $hybrid_search/$bm25_search}")
    echo -e "  ${BLUE}•${NC} BM25 search:  $(format_time ${bm25_search%.*}) (text-only)"
    echo -e "  ${BLUE}•${NC} Hybrid search: $(format_time ${hybrid_search%.*}) (${speedup}x slower, but more accurate)"
fi

echo ""
echo -e "${GREEN}✓ Formatted report saved to: ${NC}$FORMATTED_OUTPUT"
echo -e "${GREEN}✓ Latest report symlinked to: ${NC}$OUTPUT_DIR/latest-report.txt"

# Create symlink to latest report
ln -sf "$(basename "$FORMATTED_OUTPUT")" "$OUTPUT_DIR/latest-report.txt"

echo ""
echo -e "${BOLD}${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
