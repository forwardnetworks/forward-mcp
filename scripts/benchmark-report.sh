#!/usr/bin/env bash
# benchmark-report.sh - Generate formatted benchmark reports
set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# Configuration
BENCHTIME="${BENCHTIME:-1s}"
OUTPUT_DIR="${BENCHMARK_OUTPUT_DIR:-./benchmark-results}"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

mkdir -p "$OUTPUT_DIR"

echo -e "${BOLD}${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BOLD}${CYAN}  Forward MCP Benchmark Report${NC}"
echo -e "${BOLD}${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${BLUE}Date:${NC}      $(date '+%Y-%m-%d %H:%M:%S')"
echo -e "${BLUE}Platform:${NC}  $(uname -s) $(uname -m)"
echo -e "${BLUE}Go:${NC}        $(go version | awk '{print $3}')"
echo ""
echo -e "${YELLOW}Running benchmarks...${NC}"
echo ""

RAW_OUTPUT="$OUTPUT_DIR/raw-$TIMESTAMP.txt"
FORMATTED_OUTPUT="$OUTPUT_DIR/report-$TIMESTAMP.txt"

# Run benchmarks (filter out logs)
CGO_ENABLED=1 go test -bench=. -run=^$ -benchtime="$BENCHTIME" -benchmem ./internal/... 2>&1 | \
    grep -v "\[INFO\]" | \
    grep -v "\[DEBUG\]" | \
    tee "$RAW_OUTPUT"

echo ""
echo -e "${GREEN}✓ Raw results saved to: ${NC}$RAW_OUTPUT"
echo ""

# Generate formatted report
{
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "AUTO-HYDRATION BENCHMARKS"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    printf "%-45s %10s  %15s  %12s  %10s\n" "Benchmark" "Iterations" "Time/op" "Memory/op" "Allocs/op"
    echo "────────────────────────────────────────────────────────────────────────────────────────────"

    grep "^Benchmark" "$RAW_OUTPUT" | grep -E "DatabaseHydration|IncrementalUpdate|QueryStore|SmartCaching|CompleteAutoHydration" | while read -r line; do
        name=$(echo "$line" | awk '{print $1}' | sed 's/Benchmark//; s/-[0-9]*$//')
        iterations=$(echo "$line" | awk '{print $2}')
        ns_per_op=$(echo "$line" | awk '{print $3}' | sed 's/ ns\/op//')
        bytes_per_op=$(echo "$line" | awk '{print $5}' | sed 's/ B\/op//')
        allocs_per_op=$(echo "$line" | awk '{print $7}' | sed 's/ allocs\/op//')

        # Simple formatting without awk
        if [ "$ns_per_op" -gt 1000000 ]; then
            time_str=$(echo "scale=2; $ns_per_op / 1000000" | bc)" ms"
        elif [ "$ns_per_op" -gt 1000 ]; then
            time_str=$(echo "scale=2; $ns_per_op / 1000" | bc)" µs"
        else
            time_str="$ns_per_op ns"
        fi

        if [ "$bytes_per_op" -gt 1048576 ]; then
            mem_str=$(echo "scale=2; $bytes_per_op / 1048576" | bc)" MB"
        elif [ "$bytes_per_op" -gt 1024 ]; then
            mem_str=$(echo "scale=2; $bytes_per_op / 1024" | bc)" KB"
        else
            mem_str="$bytes_per_op B"
        fi

        printf "%-45s %10s  %15s  %12s  %10s\n" "$name" "$iterations" "$time_str" "$mem_str" "$allocs_per_op"
    done

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "QUERY INDEX BENCHMARKS"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    printf "%-45s %10s  %15s  %12s  %10s\n" "Benchmark" "Iterations" "Time/op" "Memory/op" "Allocs/op"
    echo "────────────────────────────────────────────────────────────────────────────────────────────"

    grep "^Benchmark" "$RAW_OUTPUT" | grep -E "GenerateEmbeddings|LoadFromQueries|SearchQueries" | while read -r line; do
        name=$(echo "$line" | awk '{print $1}' | sed 's/Benchmark//; s/-[0-9]*$//')
        iterations=$(echo "$line" | awk '{print $2}')
        ns_per_op=$(echo "$line" | awk '{print $3}' | sed 's/ ns\/op//')
        bytes_per_op=$(echo "$line" | awk '{print $5}' | sed 's/ B\/op//')
        allocs_per_op=$(echo "$line" | awk '{print $7}' | sed 's/ allocs\/op//')

        if [ "$ns_per_op" -gt 1000000 ]; then
            time_str=$(echo "scale=2; $ns_per_op / 1000000" | bc)" ms"
        elif [ "$ns_per_op" -gt 1000 ]; then
            time_str=$(echo "scale=2; $ns_per_op / 1000" | bc)" µs"
        else
            time_str="$ns_per_op ns"
        fi

        if [ "$bytes_per_op" -gt 1048576 ]; then
            mem_str=$(echo "scale=2; $bytes_per_op / 1048576" | bc)" MB"
        elif [ "$bytes_per_op" -gt 1024 ]; then
            mem_str=$(echo "scale=2; $bytes_per_op / 1024" | bc)" KB"
        else
            mem_str="$bytes_per_op B"
        fi

        printf "%-45s %10s  %15s  %12s  %10s\n" "$name" "$iterations" "$time_str" "$mem_str" "$allocs_per_op"
    done

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

} | tee "$FORMATTED_OUTPUT"

# Extract key metrics
echo -e "${BOLD}${CYAN}Key Insights:${NC}"
echo ""

bm25_line=$(grep "BenchmarkSearchQueries_BM25-" "$RAW_OUTPUT" | head -1)
hybrid_line=$(grep "BenchmarkSearchQueries_Hybrid-" "$RAW_OUTPUT" | head -1)

if [ -n "$bm25_line" ] && [ -n "$hybrid_line" ]; then
    bm25_ns=$(echo "$bm25_line" | awk '{print $3}' | sed 's/ ns\/op//')
    hybrid_ns=$(echo "$hybrid_line" | awk '{print $3}' | sed 's/ ns\/op//')

    if [ "$bm25_ns" -gt 1000 ]; then
        bm25_display=$(echo "scale=2; $bm25_ns / 1000" | bc)" µs"
    else
        bm25_display="$bm25_ns ns"
    fi

    if [ "$hybrid_ns" -gt 1000000 ]; then
        hybrid_display=$(echo "scale=2; $hybrid_ns / 1000000" | bc)" ms"
    elif [ "$hybrid_ns" -gt 1000 ]; then
        hybrid_display=$(echo "scale=2; $hybrid_ns / 1000" | bc)" µs"
    else
        hybrid_display="$hybrid_ns ns"
    fi

    speedup=$(echo "scale=1; $hybrid_ns / $bm25_ns" | bc)

    echo -e "  ${BLUE}•${NC} BM25 search (text-only):     $bm25_display per search"
    echo -e "  ${BLUE}•${NC} Hybrid search (BM25+embed):  $hybrid_display per search (${speedup}x slower, more accurate)"
fi

save_line=$(grep "SaveQueries-" "$RAW_OUTPUT" | head -1)
load_line=$(grep "LoadQueries-" "$RAW_OUTPUT" | head -1)

if [ -n "$save_line" ] && [ -n "$load_line" ]; then
    save_ns=$(echo "$save_line" | awk '{print $3}' | sed 's/ ns\/op//')
    load_ns=$(echo "$load_line" | awk '{print $3}' | sed 's/ ns\/op//')

    save_display=$(echo "scale=2; $save_ns / 1000" | bc)" µs"
    load_display=$(echo "scale=2; $load_ns / 1000" | bc)" µs"

    echo -e "  ${BLUE}•${NC} Database save: $save_display per operation"
    echo -e "  ${BLUE}•${NC} Database load: $load_display per operation"
fi

echo ""
echo -e "${GREEN}✓ Formatted report saved to: ${NC}$FORMATTED_OUTPUT"
echo -e "${GREEN}✓ Latest report symlinked to: ${NC}$OUTPUT_DIR/latest-report.txt"

ln -sf "$(basename "$FORMATTED_OUTPUT")" "$OUTPUT_DIR/latest-report.txt"

echo ""
echo -e "${BOLD}${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
